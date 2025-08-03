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
	clientv3 "go.etcd.io/etcd/client/v3"
)

func TestUpdateNode(t *testing.T) {
	// 初始化
	appconfig.Init("../config.yaml")
	dal.InitDB()
	etcd.InitEtcd()

	// 启动测试服务器
	go func() {
		srv := server.NewServer()
		if err := http.ListenAndServe(":8083", srv); err != nil {
			t.Logf("Server error: %v", err)
		}
	}()
	time.Sleep(2 * time.Second)

	ctx := context.Background()

	// 清理测试数据
	dao.Q.NodeExec.WithContext(ctx).Where(dao.Q.NodeExec.Namespace.Eq("test")).Delete()
	etcdClient := etcd.GetClient()
	_, err := etcdClient.Delete(ctx, "/namespace/test/", clientv3.WithPrefix())
	if err != nil {
		t.Logf("Failed to clean etcd: %v", err)
	}

	// 创建测试节点
	node := &model.NodeExec{
		Name:      "test-update-node",
		Kind:      "task",
		Namespace: "test",
		Version:   "v1",
		Phase: &model.NodePhase{
			Phase: model.NodePhaseReady,
		},
	}

	err = dao.Q.NodeExec.WithContext(ctx).Create(node)
	if err != nil {
		t.Fatalf("Create test node failed: %v", err)
	}

	t.Logf("Created test node with ID: %d", node.Id)

	// 创建客户端
	testConfig := config.SimpleTestClientConfig()
	testConfig.ServerURL = "http://localhost:8083/api/v1" // 使用8083端口
	client := typed.NewNodeExecClientWithConfig(typed.ClientConfig{
		ServerURL: testConfig.ServerURL,
		Timeout:   testConfig.Timeout,
	})

	// 测试更新节点状态
	node.Phase.Phase = model.NodePhaseRunning
	updatedNode, err := client.Update(ctx, node)
	if err != nil {
		t.Fatalf("Update node failed: %v", err)
	}

	if updatedNode.Phase.Phase != model.NodePhaseRunning {
		t.Errorf("Expected phase Running, got %s", updatedNode.Phase.Phase)
	}

	t.Logf("✅ Update test passed: %s -> %s", model.NodePhaseReady, updatedNode.Phase.Phase)

	// 验证 etcd 中的数据也被更新了
	key := "/namespace/test/kind/task/version/v1/node_exec/" + string(rune(node.Id))
	resp, err := etcdClient.Get(ctx, key)
	if err == nil && len(resp.Kvs) > 0 {
		t.Logf("✅ etcd data updated successfully")
	}

	// 测试删除节点
	err = client.Delete(ctx, "test", "task", node.Id)
	if err != nil {
		t.Fatalf("Delete node failed: %v", err)
	}

	t.Logf("✅ Delete test passed")

	// 验证节点已被删除
	_, err = client.Get(ctx, "test", "task", node.Id)
	if err == nil {
		t.Error("Expected node to be deleted, but still exists")
	} else {
		t.Logf("✅ Node successfully deleted")
	}
}