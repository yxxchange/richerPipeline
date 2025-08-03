package test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	client_pipe "github.com/yxxchange/pipefree/client_pipe"
	"github.com/yxxchange/pipefree/client_pipe/informers"
	appconfig "github.com/yxxchange/pipefree/config"
	server "github.com/yxxchange/pipefree/http"
	"github.com/yxxchange/pipefree/http/api/pipe_cfg"
	"github.com/yxxchange/pipefree/infra/dal"
	"github.com/yxxchange/pipefree/infra/dal/model"
	"github.com/yxxchange/pipefree/infra/etcd"
	"gopkg.in/yaml.v3"
	"testing"
)

func TestMain(m *testing.M) {
	appconfig.Init("../config.yaml")
	dal.InitDB()
	etcd.InitEtcd()

	go func() {
		err := server.LaunchServer()
		if err != nil {
			panic(err)
		}
	}()

	time.Sleep(2 * time.Second)
	m.Run()
}

func TestSimpleLinearPipeline(t *testing.T) {
	// 1. 读取流水线配置
	pipelineData, err := os.ReadFile("pipelines/simple-linear.yaml")
	if err != nil {
		t.Fatalf("读取流水线配置失败: %v", err)
	}

	var yamlConfig struct {
		Metadata struct {
			Name  string `yaml:"name"`
			Space string `yaml:"space"`
		} `yaml:"metadata"`
		Spec struct {
			Description string `yaml:"description"`
			Nodes       []struct {
				Name      string                 `yaml:"name"`
				Kind      string                 `yaml:"kind"`
				Namespace string                 `yaml:"namespace"`
				Spec      map[string]interface{} `yaml:"spec"`
			} `yaml:"nodes"`
			Graph struct {
				Edges []model.Edge `yaml:"edges"`
			} `yaml:"graph"`
			EnvVars []model.EnvVar `yaml:"envVars"`
		} `yaml:"spec"`
	}
	yaml.Unmarshal(pipelineData, &yamlConfig)

	// 2. 创建流水线配置
	pipeView := pipe_cfg.PipeView{
		PipeCfg: &model.PipeCfg{
			Name:    yamlConfig.Metadata.Name,
			Space:   yamlConfig.Metadata.Space,
			Desc:    yamlConfig.Spec.Description,
			Version: 1,
			EnvVars: (*model.EnvVars)(&yamlConfig.Spec.EnvVars),
			Graph:   &model.Graph{Edges: yamlConfig.Spec.Graph.Edges},
		},
		NodeCfgList: make([]*model.NodeCfg, 0, len(yamlConfig.Spec.Nodes)),
	}

	for _, node := range yamlConfig.Spec.Nodes {
		nodeCfg := &model.NodeCfg{
			Name:      node.Name,
			Kind:      node.Kind,
			Namespace: node.Namespace,
			PipeSpace: yamlConfig.Metadata.Space,
			PipeName:  yamlConfig.Metadata.Name,
			Version:   "v1",
			Spec:      (*model.Kv)(&node.Spec),
		}
		pipeView.NodeCfgList = append(pipeView.NodeCfgList, nodeCfg)
		t.Logf("📄 创建节点配置: name=%s, kind=%s, namespace=%s, pipeSpace=%s",
			nodeCfg.Name, nodeCfg.Kind, nodeCfg.Namespace, nodeCfg.PipeSpace)
	}

	pipeId := createPipeline(t, pipeView)

	// 3. 启动流水线
	runPipeline(t, pipeId)

	// 4. 启动operator（使用Informer）
	client := client_pipe.NewClientSet()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 使用节点的 namespace 进行监听（假设所有节点都在同一个 namespace）
	nodeNamespace := yamlConfig.Spec.Nodes[0].Namespace
	go startOperator(ctx, t, client, nodeNamespace)

	// 5. 等待完成
	time.Sleep(20 * time.Second)
	cancel()
}

func createPipeline(t *testing.T, pipeView pipe_cfg.PipeView) int64 {
	reqData := pipe_cfg.PipeReqParam{View: pipeView}
	jsonData, _ := json.Marshal(reqData)

	resp, err := http.Post("http://localhost:8080/api/v1/pipe_cfg", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		t.Fatalf("创建流水线失败: %v", err)
	}
	defer resp.Body.Close()

	var result struct {
		Code   int    `json:"code"`
		ErrMsg string `json:"err_msg"`
		Info   struct {
			Message string `json:"message"`
			PipeId  int64  `json:"pipe_id"`
		} `json:"info"`
	}
	json.NewDecoder(resp.Body).Decode(&result)

	if result.Code != 0 {
		t.Fatalf("API错误: %s", result.ErrMsg)
	}
	t.Logf("流水线创建成功，ID: %d", result.Info.PipeId)
	return result.Info.PipeId
}

func runPipeline(t *testing.T, pipeId int64) {
	url := fmt.Sprintf("http://localhost:8080/api/v1/pipe_exec/%d", pipeId)
	resp, err := http.Post(url, "application/json", nil)
	if err != nil {
		t.Fatalf("启动流水线失败: %v", err)
	}
	defer resp.Body.Close()

	var result struct {
		Code   int    `json:"code"`
		ErrMsg string `json:"err_msg"`
	}
	json.NewDecoder(resp.Body).Decode(&result)

	if result.Code != 0 {
		t.Fatalf("API错误: %s", result.ErrMsg)
	}
	t.Logf("流水线启动成功")
}

func startOperator(ctx context.Context, t *testing.T, client client_pipe.Interface, namespace string) {
	t.Logf("[Operator] 启动operator，使用完整的Informer模式")

	// 获取Informer Factory
	factory := client.Informers()

	// 获取NodeExec Informer
	kind := "task" // 从 YAML 中获取的 kind
	t.Logf("🔍 监听参数: namespace=%s, kind=%s", namespace, kind)
	informer := factory.GetInformer(namespace, kind)

	// 添加事件处理器
	informer.AddEventHandler(informers.ResourceEventHandlerFuncs{
		AddFunc: func(obj *model.NodeExec) {
			t.Logf("[Operator] 收到Add事件: 节点 %s, 状态: %s", obj.Name, obj.Phase.Phase)
			go handleNodeEvent(ctx, t, client, obj)
		},
		UpdateFunc: func(oldObj, newObj *model.NodeExec) {
			t.Logf("[Operator] 收到Update事件: 节点 %s, 状态: %s -> %s",
				newObj.Name, oldObj.Phase.Phase, newObj.Phase.Phase)
			go handleNodeEvent(ctx, t, client, newObj)
		},
		DeleteFunc: func(obj *model.NodeExec) {
			t.Logf("[Operator] 收到Delete事件: 节点 %s", obj.Name)
		},
	})

	// 启动Informer Factory
	factory.Start(ctx)

	// 等待缓存同步
	if !factory.WaitForCacheSync(ctx) {
		t.Logf("[Operator] 等待缓存同步超时")
		return
	}

	t.Logf("[Operator] Informer缓存已同步，开始处理事件")

	// 等待停止信号
	<-ctx.Done()
	t.Logf("[Operator] operator停止")
}

func handleNodeEvent(ctx context.Context, t *testing.T, client client_pipe.Interface, nodeExec *model.NodeExec) {
	if nodeExec.Phase == nil {
		return
	}

	switch nodeExec.Phase.Phase {
	case model.NodePhaseReady:
		t.Logf("[Controller] 节点 %s 就绪，开始执行", nodeExec.Name)
		nodeExec.Phase.Phase = model.NodePhaseRunning
		client.NodeExecs().Update(ctx, nodeExec)

	case model.NodePhaseRunning:
		t.Logf("[Controller] 节点 %s 运行中，模拟执行", nodeExec.Name)
		go func() {
			time.Sleep(2 * time.Second)
			nodeExec.Phase.Phase = model.NodePhaseSucceeded
			client.NodeExecs().Update(ctx, nodeExec)
			t.Logf("[Controller] 节点 %s 执行完成", nodeExec.Name)
		}()

	case model.NodePhaseSucceeded:
		t.Logf("[Controller] 节点 %s 已完成", nodeExec.Name)
	}
}
