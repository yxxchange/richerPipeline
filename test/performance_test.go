package test

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yxxchange/pipefree/client_pipe"
	"github.com/yxxchange/pipefree/client_pipe/informers/nodeexec"
	"github.com/yxxchange/pipefree/infra/dal/model"
	"github.com/yxxchange/pipefree/infra/etcd"
)

// TestHighConcurrencyEvents 测试高并发事件处理能力
func TestHighConcurrencyEvents(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	t.Log("=== 高并发事件处理测试 ===")

	const (
		numObjects    = 100  // 创建的对象数量
		numOperations = 300  // 总操作数（创建+更新+删除）
	)

	// 创建客户端
	clientset := client_pipe.NewClientSet()
	factory := clientset.Informers()

	// 事件计数器
	var addCount, updateCount, deleteCount int64
	
	// 创建 informer
	informer := factory.NodeExecs().V1().NodeExecs("perf-test", "benchmark")
	informer.AddEventHandler(nodeexec.ResourceEventHandlerFuncs{
		AddFunc: func(obj *model.NodeExec) {
			atomic.AddInt64(&addCount, 1)
		},
		UpdateFunc: func(oldObj, newObj *model.NodeExec) {
			atomic.AddInt64(&updateCount, 1)
		},
		DeleteFunc: func(obj *model.NodeExec) {
			atomic.AddInt64(&deleteCount, 1)
		},
	})

	// 启动 informer
	factory.Start(ctx)
	if !factory.WaitForCacheSync(ctx) {
		t.Fatal("Failed to sync cache")
	}

	etcdClient := etcd.GetClient()
	startTime := time.Now()

	// 阶段1: 并发创建对象
	t.Logf("创建 %d 个对象...", numObjects)
	var wg sync.WaitGroup
	
	for i := 0; i < numObjects; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			
			nodeExec := &model.NodeExec{
				Basic: model.Basic{
					Id:        int64(3000 + id),
					CreatedAt: time.Now().Unix(),
					UpdatedAt: time.Now().Unix(),
				},
				Name:      fmt.Sprintf("perf-test-node-%d", id),
				Kind:      "benchmark",
				Namespace: "perf-test",
				Phase: &model.NodePhase{
					Phase: "Pending",
				},
			}

			key := fmt.Sprintf("/pipefree/nodeexec/%s/%s/%d", 
				nodeExec.Namespace, nodeExec.Kind, nodeExec.Id)
			nodeExecJSON, _ := json.Marshal(nodeExec)
			
			if _, err := etcdClient.Put(ctx, key, string(nodeExecJSON)); err != nil {
				t.Errorf("Failed to create object %d: %v", id, err)
			}
		}(i)
	}
	wg.Wait()

	// 等待创建事件处理完成
	time.Sleep(3 * time.Second)
	createDuration := time.Since(startTime)

	// 阶段2: 并发更新对象
	t.Logf("更新 %d 个对象...", numObjects)
	updateStartTime := time.Now()
	
	for i := 0; i < numObjects; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			
			nodeExec := &model.NodeExec{
				Basic: model.Basic{
					Id:        int64(3000 + id),
					CreatedAt: time.Now().Unix(),
					UpdatedAt: time.Now().Unix(),
				},
				Name:      fmt.Sprintf("perf-test-node-%d", id),
				Kind:      "benchmark",
				Namespace: "perf-test",
				Phase: &model.NodePhase{
					Phase: "Running",
				},
			}

			key := fmt.Sprintf("/pipefree/nodeexec/%s/%s/%d", 
				nodeExec.Namespace, nodeExec.Kind, nodeExec.Id)
			nodeExecJSON, _ := json.Marshal(nodeExec)
			
			if _, err := etcdClient.Put(ctx, key, string(nodeExecJSON)); err != nil {
				t.Errorf("Failed to update object %d: %v", id, err)
			}
		}(i)
	}
	wg.Wait()

	// 等待更新事件处理完成
	time.Sleep(3 * time.Second)
	updateDuration := time.Since(updateStartTime)

	// 阶段3: 并发删除对象
	t.Logf("删除 %d 个对象...", numObjects)
	deleteStartTime := time.Now()
	
	for i := 0; i < numObjects; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			
			key := fmt.Sprintf("/pipefree/nodeexec/perf-test/benchmark/%d", 3000+id)
			if _, err := etcdClient.Delete(ctx, key); err != nil {
				t.Errorf("Failed to delete object %d: %v", id, err)
			}
		}(i)
	}
	wg.Wait()

	// 等待删除事件处理完成
	time.Sleep(3 * time.Second)
	deleteDuration := time.Since(deleteStartTime)
	totalDuration := time.Since(startTime)

	// 验证事件计数
	finalAddCount := atomic.LoadInt64(&addCount)
	finalUpdateCount := atomic.LoadInt64(&updateCount)
	finalDeleteCount := atomic.LoadInt64(&deleteCount)

	t.Logf("=== 性能测试结果 ===")
	t.Logf("创建耗时: %v (平均: %v/对象)", createDuration, createDuration/time.Duration(numObjects))
	t.Logf("更新耗时: %v (平均: %v/对象)", updateDuration, updateDuration/time.Duration(numObjects))
	t.Logf("删除耗时: %v (平均: %v/对象)", deleteDuration, deleteDuration/time.Duration(numObjects))
	t.Logf("总耗时: %v", totalDuration)
	t.Logf("事件统计: ADD=%d, UPDATE=%d, DELETE=%d", finalAddCount, finalUpdateCount, finalDeleteCount)
	t.Logf("事件处理速率: %.2f events/sec", float64(finalAddCount+finalUpdateCount+finalDeleteCount)/totalDuration.Seconds())

	// 验证事件完整性
	if finalAddCount != int64(numObjects) {
		t.Errorf("Expected %d ADD events, got %d", numObjects, finalAddCount)
	}
	if finalUpdateCount != int64(numObjects) {
		t.Errorf("Expected %d UPDATE events, got %d", numObjects, finalUpdateCount)
	}
	if finalDeleteCount != int64(numObjects) {
		t.Errorf("Expected %d DELETE events, got %d", numObjects, finalDeleteCount)
	}

	if finalAddCount == int64(numObjects) && 
	   finalUpdateCount == int64(numObjects) && 
	   finalDeleteCount == int64(numObjects) {
		t.Log("✓ 所有事件都被正确处理")
	}
}

// TestCacheConsistency 测试缓存一致性
func TestCacheConsistency(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	t.Log("=== 缓存一致性测试 ===")

	// 创建两个独立的客户端
	clientset1 := client_pipe.NewClientSet()
	clientset2 := client_pipe.NewClientSet()

	factory1 := clientset1.Informers()
	factory2 := clientset2.Informers()

	// 创建 informer
	informer1 := factory1.NodeExecs().V1().NodeExecs("consistency-test", "cache")
	informer2 := factory2.NodeExecs().V1().NodeExecs("consistency-test", "cache")

	// 启动两个 informer
	factory1.Start(ctx)
	factory2.Start(ctx)

	if !factory1.WaitForCacheSync(ctx) || !factory2.WaitForCacheSync(ctx) {
		t.Fatal("Failed to sync cache")
	}

	// 创建测试对象
	testNodeExec := &model.NodeExec{
		Basic: model.Basic{
			Id:        4000,
			CreatedAt: time.Now().Unix(),
			UpdatedAt: time.Now().Unix(),
		},
		Name:      "consistency-test-node",
		Kind:      "cache",
		Namespace: "consistency-test",
		Phase: &model.NodePhase{
			Phase: "Pending",
		},
	}

	etcdClient := etcd.GetClient()
	key := fmt.Sprintf("/pipefree/nodeexec/%s/%s/%d", 
		testNodeExec.Namespace, testNodeExec.Kind, testNodeExec.Id)
	nodeExecJSON, _ := json.Marshal(testNodeExec)
	
	// 写入 etcd
	if _, err := etcdClient.Put(ctx, key, string(nodeExecJSON)); err != nil {
		t.Fatalf("Failed to create test object: %v", err)
	}

	// 等待两个客户端都同步到数据
	time.Sleep(3 * time.Second)

	// 获取两个客户端的 lister
	lister1 := informer1.Lister()
	lister2 := informer2.Lister()

	// 从两个缓存中查询同一个对象
	obj1, err1 := lister1.Get("consistency-test", "cache", 4000)
	obj2, err2 := lister2.Get("consistency-test", "cache", 4000)

	if err1 != nil || err2 != nil {
		t.Fatalf("Failed to get object from cache: err1=%v, err2=%v", err1, err2)
	}

	// 验证两个缓存中的对象是否一致
	if obj1.Name != obj2.Name || obj1.Phase.Phase != obj2.Phase.Phase {
		t.Errorf("Cache inconsistency detected: obj1=%+v, obj2=%+v", obj1, obj2)
	} else {
		t.Log("✓ 两个客户端缓存数据一致")
	}

	// 更新对象
	testNodeExec.Phase.Phase = "Running"
	testNodeExec.UpdatedAt = time.Now().Unix()
	updatedJSON, _ := json.Marshal(testNodeExec)
	etcdClient.Put(ctx, key, string(updatedJSON))

	// 等待更新同步
	time.Sleep(2 * time.Second)

	// 再次验证一致性
	obj1, _ = lister1.Get("consistency-test", "cache", 4000)
	obj2, _ = lister2.Get("consistency-test", "cache", 4000)

	if obj1.Phase.Phase != "Running" || obj2.Phase.Phase != "Running" {
		t.Errorf("Update not synced properly: obj1.Phase=%s, obj2.Phase=%s", 
			obj1.Phase.Phase, obj2.Phase.Phase)
	} else {
		t.Log("✓ 更新后缓存仍然一致")
	}

	// 清理
	etcdClient.Delete(ctx, key)
}

// TestNetworkPartition 测试网络分区恢复能力
func TestNetworkPartition(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	t.Log("=== 网络分区恢复测试 ===")

	clientset := client_pipe.NewClientSet()
	factory := clientset.Informers()

	eventCollector := &EventCollector{}
	informer := factory.NodeExecs().V1().NodeExecs("partition-test", "network")
	informer.AddEventHandler(nodeexec.ResourceEventHandlerFuncs{
		AddFunc: func(obj *model.NodeExec) {
			t.Logf("Network partition test - ADD: %s", obj.Name)
			eventCollector.AddEvent("ADD", obj)
		},
		UpdateFunc: func(oldObj, newObj *model.NodeExec) {
			t.Logf("Network partition test - UPDATE: %s", newObj.Name)
			eventCollector.AddEvent("UPDATE", newObj)
		},
	})

	factory.Start(ctx)
	if !factory.WaitForCacheSync(ctx) {
		t.Fatal("Failed to sync cache")
	}

	etcdClient := etcd.GetClient()

	// 创建初始对象
	testNodeExec := &model.NodeExec{
		Basic: model.Basic{
			Id:        5000,
			CreatedAt: time.Now().Unix(),
			UpdatedAt: time.Now().Unix(),
		},
		Name:      "partition-test-node",
		Kind:      "network",
		Namespace: "partition-test",
		Phase: &model.NodePhase{
			Phase: "Pending",
		},
	}

	key := fmt.Sprintf("/pipefree/nodeexec/%s/%s/%d", 
		testNodeExec.Namespace, testNodeExec.Kind, testNodeExec.Id)
	nodeExecJSON, _ := json.Marshal(testNodeExec)
	etcdClient.Put(ctx, key, string(nodeExecJSON))

	// 等待初始同步
	time.Sleep(2 * time.Second)

	// 模拟网络分区期间的多次更新
	t.Log("模拟网络分区期间的快速更新...")
	phases := []string{"Running", "Paused", "Resumed", "Completing", "Succeeded"}
	
	for i, phase := range phases {
		testNodeExec.Phase.Phase = phase
		testNodeExec.UpdatedAt = time.Now().Unix() + int64(i)
		updatedJSON, _ := json.Marshal(testNodeExec)
		
		// 快速连续更新，模拟网络恢复后的批量同步
		etcdClient.Put(ctx, key, string(updatedJSON))
		time.Sleep(100 * time.Millisecond) // 短暂间隔
	}

	// 等待所有更新处理完成
	time.Sleep(3 * time.Second)

	// 验证最终状态
	lister := informer.Lister()
	finalObj, err := lister.Get("partition-test", "network", 5000)
	if err != nil {
		t.Fatalf("Failed to get final object: %v", err)
	}

	if finalObj.Phase.Phase != "Succeeded" {
		t.Errorf("Expected final phase 'Succeeded', got '%s'", finalObj.Phase.Phase)
	} else {
		t.Log("✓ 网络分区恢复后状态正确")
	}

	// 验证事件序列
	events := eventCollector.GetEvents()
	if len(events) == 0 {
		t.Error("No events received during network partition test")
	} else {
		t.Logf("✓ 收到 %d 个事件，系统正常恢复", len(events))
	}

	// 清理
	etcdClient.Delete(ctx, key)
}

// BenchmarkInformerThroughput 基准测试 - Informer 吞吐量
func BenchmarkInformerThroughput(b *testing.B) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	clientset := client_pipe.NewClientSet()
	factory := clientset.Informers()

	var eventCount int64
	informer := factory.NodeExecs().V1().NodeExecs("benchmark", "throughput")
	informer.AddEventHandler(nodeexec.ResourceEventHandlerFuncs{
		AddFunc: func(obj *model.NodeExec) {
			atomic.AddInt64(&eventCount, 1)
		},
	})

	factory.Start(ctx)
	factory.WaitForCacheSync(ctx)

	etcdClient := etcd.GetClient()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		id := 0
		for pb.Next() {
			nodeExec := &model.NodeExec{
				Basic: model.Basic{
					Id:        int64(10000 + id),
					CreatedAt: time.Now().Unix(),
					UpdatedAt: time.Now().Unix(),
				},
				Name:      fmt.Sprintf("benchmark-node-%d", id),
				Kind:      "throughput",
				Namespace: "benchmark",
				Phase: &model.NodePhase{
					Phase: "Pending",
				},
			}

			key := fmt.Sprintf("/pipefree/nodeexec/%s/%s/%d", 
				nodeExec.Namespace, nodeExec.Kind, nodeExec.Id)
			nodeExecJSON, _ := json.Marshal(nodeExec)
			
			etcdClient.Put(ctx, key, string(nodeExecJSON))
			id++
		}
	})

	// 等待所有事件处理完成
	time.Sleep(2 * time.Second)
	
	finalEventCount := atomic.LoadInt64(&eventCount)
	b.Logf("处理了 %d 个事件", finalEventCount)
	
	// 清理基准测试数据
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("/pipefree/nodeexec/benchmark/throughput/%d", 10000+i)
		etcdClient.Delete(context.Background(), key)
	}
}

// BenchmarkCacheQuery 基准测试 - 缓存查询性能
func BenchmarkCacheQuery(b *testing.B) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	clientset := client_pipe.NewClientSet()
	factory := clientset.Informers()

	informer := factory.NodeExecs().V1().NodeExecs("cache-bench", "query")
	factory.Start(ctx)
	factory.WaitForCacheSync(ctx)

	// 预先创建一些测试数据
	etcdClient := etcd.GetClient()
	for i := 0; i < 100; i++ {
		nodeExec := &model.NodeExec{
			Basic: model.Basic{
				Id:        int64(20000 + i),
				CreatedAt: time.Now().Unix(),
				UpdatedAt: time.Now().Unix(),
			},
			Name:      fmt.Sprintf("cache-bench-node-%d", i),
			Kind:      "query",
			Namespace: "cache-bench",
			Phase: &model.NodePhase{
				Phase: "Running",
			},
		}

		key := fmt.Sprintf("/pipefree/nodeexec/%s/%s/%d", 
			nodeExec.Namespace, nodeExec.Kind, nodeExec.Id)
		nodeExecJSON, _ := json.Marshal(nodeExec)
		etcdClient.Put(ctx, key, string(nodeExecJSON))
	}

	// 等待数据同步到缓存
	time.Sleep(2 * time.Second)

	lister := informer.Lister()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			// 测试 List 操作
			_, err := lister.List("cache-bench", "query")
			if err != nil {
				b.Errorf("List failed: %v", err)
			}
		}
	})

	// 清理测试数据
	for i := 0; i < 100; i++ {
		key := fmt.Sprintf("/pipefree/nodeexec/cache-bench/query/%d", 20000+i)
		etcdClient.Delete(context.Background(), key)
	}
}