package test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/yxxchange/pipefree/client_pipe"
	"github.com/yxxchange/pipefree/client_pipe/typed"
	"github.com/yxxchange/pipefree/config"
	"github.com/yxxchange/pipefree/infra/dal"
	"github.com/yxxchange/pipefree/infra/dal/dao"
	"github.com/yxxchange/pipefree/infra/dal/model"
	"github.com/yxxchange/pipefree/infra/etcd"
	server "github.com/yxxchange/pipefree/http"
	clientv3 "go.etcd.io/etcd/client/v3"
)

func TestCorrectIntegration(t *testing.T) {
	// 初始化
	config.Init("../config.yaml")
	dal.InitDB()
	etcd.InitEtcd()

	// 启动测试服务器
	go func() {
		srv := server.NewServer()
		if err := http.ListenAndServe(":8081", srv); err != nil {
			t.Logf("Server error: %v", err)
		}
	}()
	time.Sleep(2 * time.Second) // 等待服务器启动

	ctx := context.Background()
	// 使用测试端口的客户端配置
	clientset := client_pipe.NewClientSetWithTestConfig()
	client := clientset.NodeExecs()
	httpClient := &http.Client{Timeout: 10 * time.Second}

	// 清理数据库和etcd
	dao.Q.NodeExec.WithContext(ctx).Where(dao.Q.NodeExec.Namespace.Eq("test")).Delete()
	// 清理etcd中的测试数据
	etcdClient := etcd.GetClient()
	_, err := etcdClient.Delete(ctx, "/namespace/test/", clientv3.WithPrefix())
	if err != nil {
		t.Logf("Failed to clean etcd: %v", err)
	}

	// 事件收集
	var serverActions []string
	var clientEvents []string
	var mu sync.Mutex

	// 启动客户端ListAndWatch监听
	listOpts := typed.ListOptions{
		Namespace: "test",
		Kind:      "task",
	}

	watcher, err := client.Watch(ctx, typed.WatchOptions{
		Namespace: listOpts.Namespace,
		Kind:      listOpts.Kind,
	})
	if err != nil {
		t.Fatalf("ListAndWatch failed: %v", err)
	}
	defer watcher.Stop()

	// 监听事件
	go func() {
		for event := range watcher.ResultChan() {
			mu.Lock()
			eventDesc := string(event.Type)
			if event.Object != nil {
				eventDesc += " " + event.Object.Name
			}
			clientEvents = append(clientEvents, eventDesc)
			mu.Unlock()
			t.Logf("📥 Client: %s", eventDesc)
		}
	}()

	time.Sleep(500 * time.Millisecond) // 等待监听启动

	// 服务端操作：通过API创建节点（模拟创建流水线配置和执行）
	nodes := []string{"build", "test", "deploy"}

	// 1. 先创建流水线配置（简化版，直接插入数据库）
	pipeCfg := &model.PipeCfg{
		Name:    "test-pipeline",
		Space:   "test",
		Version: 1,
	}
	err = dao.Q.PipeCfg.WithContext(ctx).Create(pipeCfg)
	if err != nil {
		t.Fatalf("Create pipe config failed: %v", err)
	}

	// 2. 创建节点配置
	for i, name := range nodes {
		nodeCfg := &model.NodeCfg{
			Name:      name,
			Kind:      "task",
			Namespace: "test",
			Version:   "v1",
			PipeSpace: "test",
			PipeName:  "test-pipeline",
			PipeCfgId: pipeCfg.Id,
			InDegree:  0, // 简化为并行执行
			Spec:      &model.Kv{},
		}
		err = dao.Q.NodeCfg.WithContext(ctx).Create(nodeCfg)
		if err != nil {
			t.Fatalf("Create node config %d failed: %v", i, err)
		}
	}

	// 3. 通过服务端API启动流水线执行
	url := fmt.Sprintf("http://localhost:8081/api/v1/pipe_exec/%d", pipeCfg.Id)
	resp, err := httpClient.Post(url, "application/json", nil)
	if err != nil {
		t.Fatalf("Start pipeline failed: %v", err)
	}
	resp.Body.Close()

	mu.Lock()
	serverActions = append(serverActions, "CREATE PIPELINE")
	mu.Unlock()
	t.Logf("🏭 Server: CREATE PIPELINE")
	time.Sleep(1 * time.Second) // 等待节点创建

	// 模拟节点状态变化（实际应该由执行引擎处理）
	// 这里简化为直接更新数据库和etcd来模拟状态变化
	phases := []string{model.NodePhasePending, model.NodePhaseRunning, model.NodePhaseSucceeded}

	// 获取创建的节点
	nodeExecs, err := dao.Q.NodeExec.WithContext(ctx).Where(
		dao.Q.NodeExec.Namespace.Eq("test"),
		dao.Q.NodeExec.Kind.Eq("task"),
	).Find()
	if err != nil {
		t.Fatalf("Get node execs failed: %v", err)
	}

	for _, phase := range phases {
		for _, node := range nodeExecs {
			// 通过服务端API更新状态（这里简化为直接数据库操作）
			node.Phase.Phase = phase
			_, err = dao.Q.NodeExec.WithContext(ctx).Where(dao.Q.NodeExec.Id.Eq(node.Id)).Updates(node)
			if err != nil {
				t.Logf("Update node failed: %v", err)
				continue
			}

			// 同步到etcd（模拟服务端行为）
			key := fmt.Sprintf("/pipefree/nodeexec/%s/%s/%d", node.Namespace, node.Kind, node.Id)
			value, _ := json.Marshal(node)
			etcd.Put(ctx, key, string(value))

			mu.Lock()
			serverActions = append(serverActions, "UPDATE "+node.Name+" "+phase)
			mu.Unlock()

			t.Logf("🏭 Server: UPDATE %s %s", node.Name, phase)
			time.Sleep(300 * time.Millisecond)
		}
	}

	// 等待事件传播
	time.Sleep(2 * time.Second)

	// 验证结果
	mu.Lock()
	serverCount := len(serverActions)
	clientCount := len(clientEvents)
	mu.Unlock()

	t.Logf("📊 Server actions: %d", serverCount)
	t.Logf("📊 Client events: %d", clientCount)

	// 完整性检查
	if clientCount < serverCount {
		t.Errorf("❌ Completeness failed: client (%d) < server (%d)", clientCount, serverCount)
	} else {
		t.Logf("✅ Completeness: OK")
	}

	// 一致性检查
	opts := typed.ListOptions{Namespace: "test", Kind: "task"}
	list, err := client.List(ctx, opts)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(list.Items) != 3 {
		t.Errorf("❌ Consistency failed: expected 3 nodes, got %d", len(list.Items))
	} else {
		allSucceeded := true
		for _, node := range list.Items {
			if node.Phase.Phase != model.NodePhaseSucceeded {
				allSucceeded = false
				break
			}
		}
		if allSucceeded {
			t.Logf("✅ Consistency: OK")
		} else {
			t.Errorf("❌ Consistency failed: not all nodes succeeded")
		}
	}

	t.Log("🎯 Test completed")
}