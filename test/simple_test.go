package test

import (
	"context"
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
)

func TestSimpleClientServer(t *testing.T) {
	// 初始化
	appconfig.Init("../config.yaml")
	dal.InitDB()
	etcd.InitEtcd()

	// 启动测试服务器
	go func() {
		srv := server.NewServer()
		if err := http.ListenAndServe(":8082", srv); err != nil {
			t.Logf("Server error: %v", err)
		}
	}()
	time.Sleep(2 * time.Second) // 等待服务器启动

	ctx := context.Background()

	// 清理测试数据
	dao.Q.NodeExec.WithContext(ctx).Where(dao.Q.NodeExec.Namespace.Eq("test")).Delete()

	// 直接在数据库中创建一个节点用于测试
	node := &model.NodeExec{
		Name:      "test-node",
		Kind:      "task",
		Namespace: "test",
		Version:   "v1",
		Phase: &model.NodePhase{
			Phase: model.NodePhaseReady,
		},
	}

	err := dao.Q.NodeExec.WithContext(ctx).Create(node)
	if err != nil {
		t.Fatalf("Create test node failed: %v", err)
	}

	t.Logf("Created test node with ID: %d", node.Id)

	// 测试客户端 List 功能
	// 直接创建使用测试端口的客户端
	testConfig := config.SimpleTestClientConfig()
	nodeExecClient := typed.NewNodeExecClientWithConfig(typed.ClientConfig{
		ServerURL: testConfig.ServerURL,
		Timeout:   testConfig.Timeout,
	})
	client := nodeExecClient

	opts := typed.ListOptions{
		Namespace: "test",
		Kind:      "task",
	}

	list, err := client.List(ctx, opts)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	t.Logf("List returned %d items", len(list.Items))
	
	if len(list.Items) == 0 {
		t.Error("Expected at least 1 item, got 0")
	} else {
		t.Logf("✅ List test passed")
		for _, item := range list.Items {
			t.Logf("  - Node: %s, Phase: %s", item.Name, item.Phase.Phase)
		}
	}
}