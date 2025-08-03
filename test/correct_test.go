package test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/yxxchange/pipefree/client/typed"
	"github.com/yxxchange/pipefree/config"
	"github.com/yxxchange/pipefree/infra/dal"
	"github.com/yxxchange/pipefree/infra/dal/dao"
	"github.com/yxxchange/pipefree/infra/dal/model"
	"github.com/yxxchange/pipefree/infra/etcd"
	"github.com/yxxchange/pipefree/service/pipe_exec"
)

func TestCorrectIntegration(t *testing.T) {
	// 初始化
	config.Init("../config.yaml")
	dal.InitDB()
	etcd.InitEtcd()

	ctx := context.Background()
	client := typed.NewNodeExecClient()

	// 清理
	dao.Q.NodeExec.WithContext(ctx).Where(dao.Q.NodeExec.Namespace.Eq("test")).Delete()

	// 事件收集
	var serverActions []string
	var clientEvents []string
	var mu sync.Mutex

	// 启动客户端ListAndWatch监听
	listOpts := typed.ListOptions{
		Namespace: "test",
		Kind:      "task",
	}

	watcher, err := client.ListAndWatch(ctx, listOpts)
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

	// 服务端操作：创建3个节点
	nodes := []string{"build", "test", "deploy"}
	var nodeExecs []*model.NodeExec

	for _, name := range nodes {
		// 1. 创建节点
		node := &model.NodeExec{
			Name:      name,
			Kind:      "task",
			Namespace: "test",
			Version:   "v1",
			Phase: &model.NodePhase{
				Phase: model.NodePhaseReady,
			},
		}

		err := dao.Q.NodeExec.WithContext(ctx).Create(node)
		if err != nil {
			t.Fatalf("Create failed: %v", err)
		}

		nodeExecs = append(nodeExecs, node)

		// 2. 写入etcd
		key := pipe_exec.KeyGen(node)
		value, _ := pipe_exec.ValueGen(node)
		etcd.Put(ctx, key, value)

		mu.Lock()
		serverActions = append(serverActions, "CREATE "+name)
		mu.Unlock()

		t.Logf("🏭 Server: CREATE %s", name)
		time.Sleep(300 * time.Millisecond)
	}

	// 服务端操作：更新节点状态
	phases := []string{model.NodePhasePending, model.NodePhaseRunning, model.NodePhaseSucceeded}

	for _, phase := range phases {
		for _, node := range nodeExecs {
			// 更新状态
			node.Phase.Phase = phase
			dao.Q.NodeExec.WithContext(ctx).Where(dao.Q.NodeExec.Id.Eq(node.Id)).Updates(node)

			// 更新etcd
			key := pipe_exec.KeyGen(node)
			value, _ := pipe_exec.ValueGen(node)
			etcd.Put(ctx, key, value)

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