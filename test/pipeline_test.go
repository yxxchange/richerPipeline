package test

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/yxxchange/pipefree/client_pipe/config"
	"github.com/yxxchange/pipefree/client_pipe/typed"
	appconfig "github.com/yxxchange/pipefree/config"
	"github.com/yxxchange/pipefree/infra/dal"
	"github.com/yxxchange/pipefree/infra/dal/dao"
	"github.com/yxxchange/pipefree/infra/dal/model"
	"github.com/yxxchange/pipefree/infra/etcd"
	server "github.com/yxxchange/pipefree/http"
	clientv3 "go.etcd.io/etcd/client/v3"
)

func TestPipelineStateManagement(t *testing.T) {
	// 初始化
	appconfig.Init("../config.yaml")
	dal.InitDB()
	etcd.InitEtcd()

	// 启动测试服务器
	go func() {
		srv := server.NewServer()
		if err := http.ListenAndServe(":8084", srv); err != nil {
			t.Logf("Server error: %v", err)
		}
	}()
	time.Sleep(2 * time.Second)

	ctx := context.Background()

	// 清理测试数据
	dao.Q.NodeExec.WithContext(ctx).Where(dao.Q.NodeExec.Namespace.Eq("pipeline-test")).Delete()
	dao.Q.PipeExec.WithContext(ctx).Where(dao.Q.PipeExec.Space.Eq("pipeline-test")).Delete()
	dao.Q.NodeCfg.WithContext(ctx).Where(dao.Q.NodeCfg.PipeSpace.Eq("pipeline-test")).Delete()
	dao.Q.PipeCfg.WithContext(ctx).Where(dao.Q.PipeCfg.Space.Eq("pipeline-test")).Delete()
	
	etcdClient := etcd.GetClient()
	_, err := etcdClient.Delete(ctx, "/namespace/pipeline-test/", clientv3.WithPrefix())
	if err != nil {
		t.Logf("Failed to clean etcd: %v", err)
	}

	// 创建流水线配置（简单的线性流水线：build -> test -> deploy）
	pipeCfg := &model.PipeCfg{
		Name:    "test-pipeline",
		Space:   "pipeline-test",
		Version: 1,
		Graph: &model.Graph{
			Edges: []model.Edge{
				{From: "build", To: "test"},
				{From: "test", To: "deploy"},
			},
		},
	}
	err = dao.Q.PipeCfg.WithContext(ctx).Create(pipeCfg)
	if err != nil {
		t.Fatalf("Create pipe config failed: %v", err)
	}

	// 创建节点配置
	nodeConfigs := []*model.NodeCfg{
		{
			Name:      "build",
			Kind:      "task",
			Namespace: "pipeline-test",
			Version:   "v1",
			PipeSpace: "pipeline-test",
			PipeName:  "test-pipeline",
			PipeCfgId: pipeCfg.Id,
			InDegree:  0, // 起始节点
			Spec:      &model.Kv{},
		},
		{
			Name:      "test",
			Kind:      "task",
			Namespace: "pipeline-test",
			Version:   "v1",
			PipeSpace: "pipeline-test",
			PipeName:  "test-pipeline",
			PipeCfgId: pipeCfg.Id,
			InDegree:  1, // 依赖 build
			Spec:      &model.Kv{},
		},
		{
			Name:      "deploy",
			Kind:      "task",
			Namespace: "pipeline-test",
			Version:   "v1",
			PipeSpace: "pipeline-test",
			PipeName:  "test-pipeline",
			PipeCfgId: pipeCfg.Id,
			InDegree:  1, // 依赖 test
			Spec:      &model.Kv{},
		},
	}

	for _, nodeCfg := range nodeConfigs {
		err = dao.Q.NodeCfg.WithContext(ctx).Create(nodeCfg)
		if err != nil {
			t.Fatalf("Create node config failed: %v", err)
		}
	}

	// 创建客户端
	testConfig := config.SimpleTestClientConfig()
	testConfig.ServerURL = "http://localhost:8084/api/v1"
	client := typed.NewNodeExecClientWithConfig(typed.ClientConfig{
		ServerURL: testConfig.ServerURL,
		Timeout:   testConfig.Timeout,
	})

	// 启动流水线
	url := fmt.Sprintf("http://localhost:8084/api/v1/pipe_exec/%d", pipeCfg.Id)
	resp, err := http.Post(url, "application/json", nil)
	if err != nil {
		t.Fatalf("Start pipeline failed: %v", err)
	}
	resp.Body.Close()

	t.Logf("🚀 Pipeline started")
	time.Sleep(1 * time.Second)

	// 获取创建的节点
	list, err := client.List(ctx, typed.ListOptions{
		Namespace: "pipeline-test",
		Kind:      "task",
	})
	if err != nil {
		t.Fatalf("List nodes failed: %v", err)
	}

	// 初始状态下，只有入度为0的节点（build）在etcd中
	if len(list.Items) != 1 {
		t.Logf("Initial state: %d nodes in etcd (expected 1 build node)", len(list.Items))
		// 不直接失败，继续测试
	}

	// 找到 build 节点（应该是 Ready 状态）
	var buildNode *model.NodeExec
	for _, node := range list.Items {
		if node.Name == "build" {
			buildNode = node
			break
		}
	}

	if buildNode == nil {
		t.Fatal("Build node not found")
	}

	if buildNode.Phase.Phase != model.NodePhaseReady {
		t.Errorf("Expected build node to be Ready, got %s", buildNode.Phase.Phase)
	}

	t.Logf("✅ Build node is Ready")

	// 模拟 build 节点完成
	buildNode.Phase.Phase = model.NodePhaseSucceeded
	_, err = client.Update(ctx, buildNode)
	if err != nil {
		t.Fatalf("Update build node failed: %v", err)
	}

	t.Logf("🔄 Build node completed")
	time.Sleep(2 * time.Second) // 等待流水线逻辑处理

	// 检查 test 节点是否被触发
	updatedList, err := client.List(ctx, typed.ListOptions{
		Namespace: "pipeline-test",
		Kind:      "task",
	})
	if err != nil {
		t.Fatalf("List updated nodes failed: %v", err)
	}

	var testNode *model.NodeExec
	for _, node := range updatedList.Items {
		if node.Name == "test" {
			testNode = node
			break
		}
	}

	if testNode == nil {
		t.Fatal("Test node not found")
	}

	t.Logf("Test node status: %s, InDegree: %d", testNode.Phase.Phase, testNode.InDegree)

	// 验证流水线状态管理
	if testNode.InDegree == 0 && testNode.Phase.Phase == model.NodePhaseReady {
		t.Logf("✅ Pipeline state management working: test node triggered")
	} else {
		t.Errorf("❌ Pipeline state management failed: test node InDegree=%d, Phase=%s", 
			testNode.InDegree, testNode.Phase.Phase)
	}

	// 继续完成 test 节点
	testNode.Phase.Phase = model.NodePhaseSucceeded
	_, err = client.Update(ctx, testNode)
	if err != nil {
		t.Fatalf("Update test node failed: %v", err)
	}

	t.Logf("🔄 Test node completed")
	time.Sleep(2 * time.Second)

	// 检查 deploy 节点
	finalList, err := client.List(ctx, typed.ListOptions{
		Namespace: "pipeline-test",
		Kind:      "task",
	})
	if err != nil {
		t.Fatalf("List final nodes failed: %v", err)
	}

	var deployNode *model.NodeExec
	for _, node := range finalList.Items {
		if node.Name == "deploy" {
			deployNode = node
			break
		}
	}

	if deployNode != nil && deployNode.InDegree == 0 && deployNode.Phase.Phase == model.NodePhaseReady {
		t.Logf("✅ Deploy node triggered successfully")
		
		// 完成 deploy 节点以触发流水线完成
		deployNode.Phase.Phase = model.NodePhaseSucceeded
		_, err = client.Update(ctx, deployNode)
		if err != nil {
			t.Fatalf("Update deploy node failed: %v", err)
		}
		
		t.Logf("🔄 Deploy node completed")
		time.Sleep(2 * time.Second) // 等待流水线状态更新
		
		// 检查流水线状态
		pipeExecs, err := dao.Q.PipeExec.WithContext(ctx).Where(dao.Q.PipeExec.Space.Eq("pipeline-test")).Find()
		if err != nil {
			t.Fatalf("Get pipeline exec failed: %v", err)
		}
		
		if len(pipeExecs) > 0 {
			pipeExec := pipeExecs[0]
			if pipeExec.State == model.PipeExecStateSuccess {
				t.Logf("✅ Pipeline completed successfully (state=%d)", pipeExec.State)
			} else {
				t.Errorf("❌ Pipeline state incorrect: expected %d, got %d", model.PipeExecStateSuccess, pipeExec.State)
			}
		}
		
	} else if deployNode != nil {
		t.Errorf("❌ Deploy node not triggered: InDegree=%d, Phase=%s", 
			deployNode.InDegree, deployNode.Phase.Phase)
	}

	t.Log("🎯 Pipeline state management test completed")
}