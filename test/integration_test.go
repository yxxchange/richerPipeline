package test

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/yxxchange/pipefree/client_pipe"
	"github.com/yxxchange/pipefree/client_pipe/informers/nodeexec"
	"github.com/yxxchange/pipefree/config"
	"github.com/yxxchange/pipefree/infra/dal"
	"github.com/yxxchange/pipefree/infra/dal/dao"
	"github.com/yxxchange/pipefree/infra/dal/model"
	"github.com/yxxchange/pipefree/infra/etcd"
)

// TestMain 初始化测试环境
func TestMain(m *testing.M) {
	// 初始化配置
	config.Init("../config.yaml")

	// 初始化数据库
	dal.InitDB()

	// 初始化 etcd
	etcd.InitEtcd()

	// 运行测试
	m.Run()
}

// TestServerClientInteraction 测试服务端与客户端的交互能力
func TestServerClientInteraction(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 1. 服务端先创建并运行一条流水线
	t.Log("=== 阶段1: 服务端创建流水线 ===")

	// 创建测试用的 NodeExec 对象
	testNodeExec := &model.NodeExec{
		Basic: model.Basic{
			CreatedAt: time.Now().Unix(),
			UpdatedAt: time.Now().Unix(),
		},
		Name:      "test-pipeline-node",
		Kind:      "pipeline",
		Namespace: "test-integration",
		Version:   "v1.0.0",
		Phase: &model.NodePhase{
			Phase: "Pending",
		},
		Spec: &model.Kv{},
	}

	// 保存到数据库
	if err := dao.Q.NodeExec.WithContext(ctx).Create(testNodeExec); err != nil {
		t.Fatalf("Failed to create test NodeExec in database: %v", err)
	}
	t.Logf("Created NodeExec in database: ID=%d, Name=%s", testNodeExec.Id, testNodeExec.Name)

	// 模拟服务端将数据写入 etcd
	etcdClient := etcd.GetClient()
	key := fmt.Sprintf("/pipefree/nodeexec/%s/%s/%d", testNodeExec.Namespace, testNodeExec.Kind, testNodeExec.Id)

	nodeExecJSON, err := json.Marshal(testNodeExec)
	if err != nil {
		t.Fatalf("Failed to marshal NodeExec: %v", err)
	}

	if _, err := etcdClient.Put(ctx, key, string(nodeExecJSON)); err != nil {
		t.Fatalf("Failed to put NodeExec to etcd: %v", err)
	}
	t.Logf("Put NodeExec to etcd: key=%s", key)

	// 2. 等待一段时间，模拟流水线已经在运行
	time.Sleep(2 * time.Second)

	// 3. 客户端在节点开始运行时才启动
	t.Log("=== 阶段2: 客户端启动并监听 ===")

	clientset := client_pipe.NewClientSet()
	factory := clientset.Informers()

	// 创建事件收集器
	eventCollector := &EventCollector{}

	// 获取 informer 并添加事件处理器
	informer := factory.NodeExecs().V1().NodeExecs(testNodeExec.Namespace, testNodeExec.Kind)
	informer.AddEventHandler(nodeexec.ResourceEventHandlerFuncs{
		AddFunc: func(obj *model.NodeExec) {
			t.Logf("Client received ADD event: %s/%s/%d (Phase: %s)",
				obj.Namespace, obj.Kind, obj.Id, obj.Phase.Phase)
			eventCollector.AddEvent("ADD", obj)
		},
		UpdateFunc: func(oldObj, newObj *model.NodeExec) {
			t.Logf("Client received UPDATE event: %s/%s/%d (Phase: %s -> %s)",
				newObj.Namespace, newObj.Kind, newObj.Id, oldObj.Phase.Phase, newObj.Phase.Phase)
			eventCollector.AddEvent("UPDATE", newObj)
		},
		DeleteFunc: func(obj *model.NodeExec) {
			t.Logf("Client received DELETE event: %s/%s/%d",
				obj.Namespace, obj.Kind, obj.Id)
			eventCollector.AddEvent("DELETE", obj)
		},
	})

	// 启动 informer
	factory.Start(ctx)

	// 等待缓存同步
	t.Log("Waiting for cache sync...")
	if !factory.WaitForCacheSync(ctx) {
		t.Fatal("Failed to sync cache")
	}
	t.Log("Cache synced successfully")

	// 4. 验证客户端能否监听到已存在的对象
	t.Log("=== 阶段3: 验证初始同步 ===")

	// 等待事件处理
	time.Sleep(1 * time.Second)

	// 检查是否收到了 ADD 事件
	events := eventCollector.GetEvents()
	if len(events) == 0 {
		t.Error("Expected to receive ADD event for existing NodeExec, but got none")
	} else {
		found := false
		for _, event := range events {
			if event.Type == "ADD" && event.Object.Id == testNodeExec.Id {
				found = true
				break
			}
		}
		if !found {
			t.Error("Expected to receive ADD event for the test NodeExec")
		} else {
			t.Log("✓ Successfully received ADD event for existing NodeExec")
		}
	}

	// 5. 模拟服务端更新流水线状态
	t.Log("=== 阶段4: 服务端更新流水线状态 ===")

	// 更新 NodeExec 状态
	testNodeExec.Phase.Phase = "Running"
	testNodeExec.UpdatedAt = time.Now().Unix()

	// 更新数据库
	if err := dao.Q.NodeExec.WithContext(ctx).Save(testNodeExec); err != nil {
		t.Fatalf("Failed to update NodeExec in database: %v", err)
	}

	// 更新 etcd
	updatedJSON, err := json.Marshal(testNodeExec)
	if err != nil {
		t.Fatalf("Failed to marshal updated NodeExec: %v", err)
	}

	if _, err := etcdClient.Put(ctx, key, string(updatedJSON)); err != nil {
		t.Fatalf("Failed to update NodeExec in etcd: %v", err)
	}
	t.Logf("Updated NodeExec phase to 'Running'")

	// 等待客户端接收更新事件
	time.Sleep(2 * time.Second)

	// 6. 验证客户端能否监听到更新事件
	t.Log("=== 阶段5: 验证更新事件 ===")

	events = eventCollector.GetEvents()
	updateEventFound := false
	for _, event := range events {
		if event.Type == "UPDATE" && event.Object.Id == testNodeExec.Id {
			if event.Object.Phase.Phase == "Running" {
				updateEventFound = true
				break
			}
		}
	}

	if !updateEventFound {
		t.Error("Expected to receive UPDATE event with phase 'Running'")
	} else {
		t.Log("✓ Successfully received UPDATE event for phase change")
	}

	// 7. 再次更新状态到完成
	t.Log("=== 阶段6: 流水线完成 ===")

	testNodeExec.Phase.Phase = "Succeeded"
	testNodeExec.UpdatedAt = time.Now().Unix()

	// 更新数据库和 etcd
	if err := dao.Q.NodeExec.WithContext(ctx).Save(testNodeExec); err != nil {
		t.Fatalf("Failed to update NodeExec to Succeeded: %v", err)
	}

	succeededJSON, err := json.Marshal(testNodeExec)
	if err != nil {
		t.Fatalf("Failed to marshal succeeded NodeExec: %v", err)
	}

	if _, err := etcdClient.Put(ctx, key, string(succeededJSON)); err != nil {
		t.Fatalf("Failed to update NodeExec to Succeeded in etcd: %v", err)
	}
	t.Logf("Updated NodeExec phase to 'Succeeded'")

	// 等待事件处理
	time.Sleep(2 * time.Second)

	// 8. 验证最终状态
	t.Log("=== 阶段7: 验证最终状态 ===")

	// 使用 lister 查询缓存中的最新状态
	lister := informer.Lister()
	cachedObj, err := lister.Get(testNodeExec.Namespace, testNodeExec.Kind, testNodeExec.Id)
	if err != nil {
		t.Fatalf("Failed to get NodeExec from cache: %v", err)
	}

	if cachedObj.Phase.Phase != "Succeeded" {
		t.Errorf("Expected cached object phase to be 'Succeeded', got '%s'", cachedObj.Phase.Phase)
	} else {
		t.Log("✓ Cache contains the latest state: Succeeded")
	}

	// 9. 测试删除事件
	t.Log("=== 阶段8: 测试删除事件 ===")

	// 从 etcd 删除对象
	if _, err := etcdClient.Delete(ctx, key); err != nil {
		t.Fatalf("Failed to delete NodeExec from etcd: %v", err)
	}
	t.Logf("Deleted NodeExec from etcd")

	// 等待删除事件
	time.Sleep(2 * time.Second)

	// 验证删除事件
	events = eventCollector.GetEvents()
	deleteEventFound := false
	for _, event := range events {
		if event.Type == "DELETE" && event.Object.Id == testNodeExec.Id {
			deleteEventFound = true
			break
		}
	}

	if !deleteEventFound {
		t.Error("Expected to receive DELETE event")
	} else {
		t.Log("✓ Successfully received DELETE event")
	}

	// 10. 清理数据库
	if _, err := dao.Q.NodeExec.WithContext(ctx).Where(dao.NodeExec.Id.Eq(testNodeExec.Id)).Delete(); err != nil {
		t.Logf("Warning: Failed to cleanup test NodeExec from database: %v", err)
	}

	// 11. 输出测试总结
	t.Log("=== 测试总结 ===")
	allEvents := eventCollector.GetEvents()
	t.Logf("Total events received: %d", len(allEvents))
	for i, event := range allEvents {
		t.Logf("Event %d: %s - %s (Phase: %s)",
			i+1, event.Type, event.Object.Name, event.Object.Phase.Phase)
	}

	t.Log("✓ Server-Client interaction test completed successfully")
}

// TestClientResilience 测试客户端的容错能力
func TestClientResilience(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	t.Log("=== 测试客户端容错能力 ===")

	clientset := client_pipe.NewClientSet()
	factory := clientset.Informers()

	eventCollector := &EventCollector{}

	// 创建 informer
	informer := factory.NodeExecs().V1().NodeExecs("resilience-test", "task")
	informer.AddEventHandler(nodeexec.ResourceEventHandlerFuncs{
		AddFunc: func(obj *model.NodeExec) {
			t.Logf("Resilience test - ADD: %s", obj.Name)
			eventCollector.AddEvent("ADD", obj)
		},
		UpdateFunc: func(oldObj, newObj *model.NodeExec) {
			t.Logf("Resilience test - UPDATE: %s", newObj.Name)
			eventCollector.AddEvent("UPDATE", newObj)
		},
		DeleteFunc: func(obj *model.NodeExec) {
			t.Logf("Resilience test - DELETE: %s", obj.Name)
			eventCollector.AddEvent("DELETE", obj)
		},
	})

	// 启动 informer
	factory.Start(ctx)

	if !factory.WaitForCacheSync(ctx) {
		t.Fatal("Failed to sync cache")
	}

	// 快速创建多个对象测试批量处理能力
	etcdClient := etcd.GetClient()

	for i := 0; i < 5; i++ {
		nodeExec := &model.NodeExec{
			Basic: model.Basic{
				Id:        int64(1000 + i),
				CreatedAt: time.Now().Unix(),
				UpdatedAt: time.Now().Unix(),
			},
			Name:      fmt.Sprintf("resilience-test-node-%d", i),
			Kind:      "task",
			Namespace: "resilience-test",
			Phase: &model.NodePhase{
				Phase: "Pending",
			},
		}

		key := fmt.Sprintf("/pipefree/nodeexec/%s/%s/%d", nodeExec.Namespace, nodeExec.Kind, nodeExec.Id)
		nodeExecJSON, _ := json.Marshal(nodeExec)

		if _, err := etcdClient.Put(ctx, key, string(nodeExecJSON)); err != nil {
			t.Fatalf("Failed to put test NodeExec %d: %v", i, err)
		}
	}

	// 等待所有事件处理完成
	time.Sleep(3 * time.Second)

	// 验证是否收到了所有 ADD 事件
	events := eventCollector.GetEvents()
	addEvents := 0
	for _, event := range events {
		if event.Type == "ADD" {
			addEvents++
		}
	}

	if addEvents < 5 {
		t.Errorf("Expected at least 5 ADD events, got %d", addEvents)
	} else {
		t.Logf("✓ Successfully handled %d ADD events", addEvents)
	}

	// 清理测试数据
	for i := 0; i < 5; i++ {
		key := fmt.Sprintf("/pipefree/nodeexec/resilience-test/task/%d", 1000+i)
		etcdClient.Delete(ctx, key)
	}

	t.Log("✓ Client resilience test completed")
}

// TestConcurrentClients 测试多个客户端并发访问
func TestConcurrentClients(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	t.Log("=== 测试多客户端并发访问 ===")

	const numClients = 3
	var wg sync.WaitGroup
	collectors := make([]*EventCollector, numClients)

	// 创建测试数据
	testNodeExec := &model.NodeExec{
		Basic: model.Basic{
			Id:        2000,
			CreatedAt: time.Now().Unix(),
			UpdatedAt: time.Now().Unix(),
		},
		Name:      "concurrent-test-node",
		Kind:      "job",
		Namespace: "concurrent-test",
		Phase: &model.NodePhase{
			Phase: "Pending",
		},
	}

	// 启动多个客户端
	for i := 0; i < numClients; i++ {
		wg.Add(1)
		collectors[i] = &EventCollector{}

		go func(clientID int, collector *EventCollector) {
			defer wg.Done()

			clientset := client_pipe.NewClientSet()
			factory := clientset.Informers()

			informer := factory.NodeExecs().V1().NodeExecs(testNodeExec.Namespace, testNodeExec.Kind)
			informer.AddEventHandler(nodeexec.ResourceEventHandlerFuncs{
				AddFunc: func(obj *model.NodeExec) {
					t.Logf("Client %d - ADD: %s", clientID, obj.Name)
					collector.AddEvent("ADD", obj)
				},
				UpdateFunc: func(oldObj, newObj *model.NodeExec) {
					t.Logf("Client %d - UPDATE: %s", clientID, newObj.Name)
					collector.AddEvent("UPDATE", newObj)
				},
			})

			factory.Start(ctx)
			factory.WaitForCacheSync(ctx)

			// 等待测试完成
			<-ctx.Done()
		}(i, collectors[i])
	}

	// 等待所有客户端启动
	time.Sleep(2 * time.Second)

	// 创建测试对象
	etcdClient := etcd.GetClient()
	key := fmt.Sprintf("/pipefree/nodeexec/%s/%s/%d", testNodeExec.Namespace, testNodeExec.Kind, testNodeExec.Id)
	nodeExecJSON, _ := json.Marshal(testNodeExec)

	if _, err := etcdClient.Put(ctx, key, string(nodeExecJSON)); err != nil {
		t.Fatalf("Failed to create concurrent test NodeExec: %v", err)
	}

	// 更新对象状态
	time.Sleep(1 * time.Second)
	testNodeExec.Phase.Phase = "Running"
	testNodeExec.UpdatedAt = time.Now().Unix()
	updatedJSON, _ := json.Marshal(testNodeExec)
	etcdClient.Put(ctx, key, string(updatedJSON))

	// 等待事件处理
	time.Sleep(2 * time.Second)

	// 取消上下文，等待所有客户端退出
	cancel()
	wg.Wait()

	// 验证所有客户端都收到了事件
	for i, collector := range collectors {
		events := collector.GetEvents()
		if len(events) == 0 {
			t.Errorf("Client %d did not receive any events", i)
		} else {
			t.Logf("Client %d received %d events", i, len(events))
		}
	}

	// 清理
	etcdClient.Delete(context.Background(), key)

	t.Log("✓ Concurrent clients test completed")
}

// EventCollector 事件收集器，用于测试
type EventCollector struct {
	mu     sync.RWMutex
	events []Event
}

type Event struct {
	Type   string
	Object *model.NodeExec
	Time   time.Time
}

func (ec *EventCollector) AddEvent(eventType string, obj *model.NodeExec) {
	ec.mu.Lock()
	defer ec.mu.Unlock()

	// 创建对象副本避免并发问题
	objCopy := *obj

	ec.events = append(ec.events, Event{
		Type:   eventType,
		Object: &objCopy,
		Time:   time.Now(),
	})
}

func (ec *EventCollector) GetEvents() []Event {
	ec.mu.RLock()
	defer ec.mu.RUnlock()

	// 返回副本
	events := make([]Event, len(ec.events))
	copy(events, ec.events)
	return events
}

func (ec *EventCollector) Clear() {
	ec.mu.Lock()
	defer ec.mu.Unlock()
	ec.events = nil
}
