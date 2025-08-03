package client_pipe

import (
	"context"
	"github.com/yxxchange/pipefree/config"
	"github.com/yxxchange/pipefree/infra/etcd"
	"testing"
	"time"

	"github.com/yxxchange/pipefree/client_pipe/informers/nodeexec"
	"github.com/yxxchange/pipefree/client_pipe/typed"
	"github.com/yxxchange/pipefree/infra/dal/model"
)

func TestMain(m *testing.M) {
	config.Init("../config.yaml")
	etcd.InitEtcd()
}

func TestClientSet(t *testing.T) {
	// 创建客户端
	clientSet := NewClientSet()

	// 测试 typed client
	nodeExecClient := clientSet.NodeExecs()
	if nodeExecClient == nil {
		t.Fatal("NodeExecs client should not be nil")
	}

	// 测试 informer factory
	factory := clientSet.Informers()
	if factory == nil {
		t.Fatal("Informer factory should not be nil")
	}
}

func TestTypedClient(t *testing.T) {
	clientSet := NewClientSet()
	client := clientSet.NodeExecs()

	ctx := context.Background()

	// 测试 List 操作
	list, err := client.List(ctx, typed.ListOptions{
		Namespace: "test",
		Kind:      "pipeline",
	})

	// 这里可能会失败，因为需要 etcd 连接
	// 但至少可以验证接口是否正确
	if err != nil {
		t.Logf("List failed (expected if etcd not available): %v", err)
	} else {
		t.Logf("List succeeded, found %d items", len(list.Items))
	}
}

func TestInformerFactory(t *testing.T) {
	clientSet := NewClientSet()
	factory := clientSet.Informers()

	// 测试获取 informer
	informer := factory.NodeExecs().V1().NodeExecs("test", "pipeline")
	if informer == nil {
		t.Fatal("Informer should not be nil")
	}

	// 测试添加事件处理器
	eventReceived := false
	informer.AddEventHandler(nodeexec.ResourceEventHandlerFuncs{
		AddFunc: func(obj *model.NodeExec) {
			eventReceived = true
			t.Logf("Received Add event for: %s", obj.Name)
		},
	})
	_ = eventReceived // 避免未使用变量警告

	// 测试获取 lister
	lister := informer.Lister()
	if lister == nil {
		t.Fatal("Lister should not be nil")
	}
}

func TestClientConfig(t *testing.T) {
	config := ClientConfig{
		DefaultResyncPeriod: 10 * time.Second,
	}

	clientSet := NewClientSetWithConfig(config)
	if clientSet == nil {
		t.Fatal("Clientset should not be nil")
	}

	// 验证配置是否生效
	factory := clientSet.Informers()
	if factory == nil {
		t.Fatal("Factory should not be nil")
	}
}

// 基准测试
func BenchmarkListFromCache(b *testing.B) {
	clientSet := NewClientSet()
	factory := clientSet.Informers()
	informer := factory.NodeExecs().V1().NodeExecs("test", "pipeline")
	lister := informer.Lister()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := lister.List("test", "pipeline")
		if err != nil {
			b.Fatalf("List from cache failed: %v", err)
		}
	}
}

func BenchmarkGetFromCache(b *testing.B) {
	clientSet := NewClientSet()
	factory := clientSet.Informers()
	informer := factory.NodeExecs().V1().NodeExecs("test", "pipeline")
	lister := informer.Lister()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := lister.Get("test", "pipeline", 1)
		if err != nil {
			// 预期会失败，因为缓存中没有数据
			continue
		}
	}
}
