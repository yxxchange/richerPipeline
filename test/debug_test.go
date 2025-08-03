package test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/yxxchange/pipefree/client_pipe"
	"github.com/yxxchange/pipefree/client_pipe/typed"
	"github.com/yxxchange/pipefree/infra/dal/model"
	"github.com/yxxchange/pipefree/infra/etcd"
)

// TestDirectEtcdAccess 测试直接访问 etcd 的功能
func TestDirectEtcdAccess(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 创建测试对象
	testObj := &model.NodeExec{
		Basic: model.Basic{
			Id:        99999,
			CreatedAt: time.Now().Unix(),
			UpdatedAt: time.Now().Unix(),
		},
		Name:      "direct-test-node",
		Kind:      "test",
		Namespace: "direct-test",
		Phase: &model.NodePhase{
			Phase: "Running",
		},
		Spec: &model.Kv{},
	}

	// 直接写入 etcd
	etcdClient := etcd.GetClient()
	key := fmt.Sprintf("/pipefree/nodeexec/%s/%s/%d", testObj.Namespace, testObj.Kind, testObj.Id)
	
	objJSON, err := json.Marshal(testObj)
	if err != nil {
		t.Fatalf("Failed to marshal object: %v", err)
	}

	if _, err := etcdClient.Put(ctx, key, string(objJSON)); err != nil {
		t.Fatalf("Failed to put to etcd: %v", err)
	}
	t.Logf("Put object to etcd: %s", key)

	// 使用 typed client 读取
	clientset := client_pipe.NewClientSet()
	client := clientset.NodeExecs()

	// 测试 Get
	retrievedObj, err := client.Get(ctx, testObj.Namespace, testObj.Kind, testObj.Id)
	if err != nil {
		t.Fatalf("Failed to get object: %v", err)
	}
	t.Logf("Retrieved object: %s (Phase: %s)", retrievedObj.Name, retrievedObj.Phase.Phase)

	// 测试 List
	list, err := client.List(ctx, typed.ListOptions{
		Namespace: testObj.Namespace,
		Kind:      testObj.Kind,
	})
	if err != nil {
		t.Fatalf("Failed to list objects: %v", err)
	}
	t.Logf("Listed %d objects", len(list.Items))

	// 清理
	if _, err := etcdClient.Delete(ctx, key); err != nil {
		t.Logf("Warning: Failed to cleanup: %v", err)
	}

	t.Log("✓ Direct etcd access test passed")
}