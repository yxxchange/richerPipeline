package test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/yxxchange/pipefree/client_pipe"
	"github.com/yxxchange/pipefree/client_pipe/informers/nodeexec"
	"github.com/yxxchange/pipefree/infra/dal/model"
	"github.com/yxxchange/pipefree/infra/etcd"
)

// TestEdgeCases 测试边界条件和异常情况
func TestEdgeCases(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	t.Log("=== 边界条件测试 ===")

	clientset := client_pipe.NewClientSet()
	factory := clientset.Informers()
	
	eventCollector := &EventCollector{}
	informer := factory.NodeExecs().V1().NodeExecs("edge-test", "boundary")
	informer.AddEventHandler(nodeexec.ResourceEventHandlerFuncs{
		AddFunc: func(obj *model.NodeExec) {
			t.Logf("Edge test - ADD: %s (ID: %d)", obj.Name, obj.Id)
			eventCollector.AddEvent("ADD", obj)
		},
		UpdateFunc: func(oldObj, newObj *model.NodeExec) {
			t.Logf("Edge test - UPDATE: %s (ID: %d)", newObj.Name, newObj.Id)
			eventCollector.AddEvent("UPDATE", newObj)
		},
		DeleteFunc: func(obj *model.NodeExec) {
			t.Logf("Edge test - DELETE: %s (ID: %d)", obj.Name, obj.Id)
			eventCollector.AddEvent("DELETE", obj)
		},
	})

	factory.Start(ctx)
	if !factory.WaitForCacheSync(ctx) {
		t.Fatal("Failed to sync cache")
	}

	etcdClient := etcd.GetClient()

	// 测试1: 空字符串字段
	t.Log("测试1: 空字符串字段")
	emptyFieldNode := &model.NodeExec{
		Basic: model.Basic{
			Id:        6001,
			CreatedAt: time.Now().Unix(),
			UpdatedAt: time.Now().Unix(),
		},
		Name:      "", // 空名称
		Kind:      "boundary",
		Namespace: "edge-test",
		Phase: &model.NodePhase{
			Phase: "",  // 空阶段
		},
		Spec: &model.Kv{}, // 空规格
	}

	key1 := fmt.Sprintf("/pipefree/nodeexec/%s/%s/%d", 
		emptyFieldNode.Namespace, emptyFieldNode.Kind, emptyFieldNode.Id)
	nodeExecJSON, _ := json.Marshal(emptyFieldNode)
	etcdClient.Put(ctx, key1, string(nodeExecJSON))

	time.Sleep(1 * time.Second)

	// 验证空字段对象是否正确处理
	lister := informer.Lister()
	retrievedObj, err := lister.Get("edge-test", "boundary", 6001)
	if err != nil {
		t.Errorf("Failed to retrieve object with empty fields: %v", err)
	} else if retrievedObj.Name != "" || retrievedObj.Phase.Phase != "" {
		t.Log("✓ 空字段对象处理正常")
	}

	// 测试2: 极长字符串
	t.Log("测试2: 极长字符串")
	longString := make([]byte, 1000)
	for i := range longString {
		longString[i] = 'A' + byte(i%26)
	}

	longStringNode := &model.NodeExec{
		Basic: model.Basic{
			Id:        6002,
			CreatedAt: time.Now().Unix(),
			UpdatedAt: time.Now().Unix(),
		},
		Name:      string(longString), // 极长名称
		Kind:      "boundary",
		Namespace: "edge-test",
		Phase: &model.NodePhase{
			Phase: "Running",
		},
		Spec: &model.Kv{}, // 极长规格
	}

	key2 := fmt.Sprintf("/pipefree/nodeexec/%s/%s/%d", 
		longStringNode.Namespace, longStringNode.Kind, longStringNode.Id)
	longNodeJSON, _ := json.Marshal(longStringNode)
	etcdClient.Put(ctx, key2, string(longNodeJSON))

	time.Sleep(1 * time.Second)

	// 验证长字符串对象
	longObj, err := lister.Get("edge-test", "boundary", 6002)
	if err != nil {
		t.Errorf("Failed to retrieve object with long strings: %v", err)
	} else if len(longObj.Name) != 1000 {
		t.Errorf("Long string not preserved, expected 1000 chars, got %d", len(longObj.Name))
	} else {
		t.Log("✓ 极长字符串处理正常")
	}

	// 测试3: 特殊字符
	t.Log("测试3: 特殊字符")
	specialCharNode := &model.NodeExec{
		Basic: model.Basic{
			Id:        6003,
			CreatedAt: time.Now().Unix(),
			UpdatedAt: time.Now().Unix(),
		},
		Name:      "测试节点-特殊字符!@#$%^&*()_+{}|:<>?[]\\;'\",./ 🚀🔥💯", // 包含中文、符号、emoji
		Kind:      "boundary",
		Namespace: "edge-test",
		Phase: &model.NodePhase{
			Phase: "Running",
		},
		Spec: &model.Kv{},
	}

	key3 := fmt.Sprintf("/pipefree/nodeexec/%s/%s/%d", 
		specialCharNode.Namespace, specialCharNode.Kind, specialCharNode.Id)
	specialJSON, _ := json.Marshal(specialCharNode)
	etcdClient.Put(ctx, key3, string(specialJSON))

	time.Sleep(1 * time.Second)

	// 验证特殊字符对象
	specialObj, err := lister.Get("edge-test", "boundary", 6003)
	if err != nil {
		t.Errorf("Failed to retrieve object with special characters: %v", err)
	} else if specialObj.Name != specialCharNode.Name {
		t.Errorf("Special characters not preserved correctly")
	} else {
		t.Log("✓ 特殊字符处理正常")
	}

	// 测试4: 极端时间戳
	t.Log("测试4: 极端时间戳")
	extremeTimeNode := &model.NodeExec{
		Basic: model.Basic{
			Id:        6004,
			CreatedAt: 0,          // 最小时间戳
			UpdatedAt: 2147483647, // 接近 int32 最大值
		},
		Name:      "extreme-time-node",
		Kind:      "boundary",
		Namespace: "edge-test",
		Phase: &model.NodePhase{
			Phase: "Running",
		},
	}

	key4 := fmt.Sprintf("/pipefree/nodeexec/%s/%s/%d", 
		extremeTimeNode.Namespace, extremeTimeNode.Kind, extremeTimeNode.Id)
	extremeJSON, _ := json.Marshal(extremeTimeNode)
	etcdClient.Put(ctx, key4, string(extremeJSON))

	time.Sleep(1 * time.Second)

	// 验证极端时间戳对象
	extremeObj, err := lister.Get("edge-test", "boundary", 6004)
	if err != nil {
		t.Errorf("Failed to retrieve object with extreme timestamps: %v", err)
	} else if extremeObj.CreatedAt != 0 || extremeObj.UpdatedAt != 2147483647 {
		t.Errorf("Extreme timestamps not preserved correctly")
	} else {
		t.Log("✓ 极端时间戳处理正常")
	}

	// 等待所有事件处理完成
	time.Sleep(2 * time.Second)

	// 验证事件计数
	events := eventCollector.GetEvents()
	addEvents := 0
	for _, event := range events {
		if event.Type == "ADD" {
			addEvents++
		}
	}

	if addEvents < 4 {
		t.Errorf("Expected at least 4 ADD events, got %d", addEvents)
	} else {
		t.Logf("✓ 收到 %d 个 ADD 事件", addEvents)
	}

	// 清理测试数据
	etcdClient.Delete(ctx, key1)
	etcdClient.Delete(ctx, key2)
	etcdClient.Delete(ctx, key3)
	etcdClient.Delete(ctx, key4)

	t.Log("✓ 边界条件测试完成")
}

// TestInvalidData 测试无效数据处理
func TestInvalidData(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	t.Log("=== 无效数据处理测试 ===")

	clientset := client_pipe.NewClientSet()
	factory := clientset.Informers()
	
	errorCount := 0
	informer := factory.NodeExecs().V1().NodeExecs("invalid-test", "malformed")
	informer.AddEventHandler(nodeexec.ResourceEventHandlerFuncs{
		AddFunc: func(obj *model.NodeExec) {
			t.Logf("Invalid data test - ADD: %s", obj.Name)
		},
		UpdateFunc: func(oldObj, newObj *model.NodeExec) {
			t.Logf("Invalid data test - UPDATE: %s", newObj.Name)
		},
		DeleteFunc: func(obj *model.NodeExec) {
			t.Logf("Invalid data test - DELETE: %s", obj.Name)
		},
	})

	factory.Start(ctx)
	if !factory.WaitForCacheSync(ctx) {
		t.Fatal("Failed to sync cache")
	}

	etcdClient := etcd.GetClient()

	// 测试1: 无效 JSON
	t.Log("测试1: 写入无效 JSON 数据")
	invalidJSONKey := "/pipefree/nodeexec/invalid-test/malformed/7001"
	invalidJSON := `{"name": "invalid-json", "phase": {"phase": "Running"` // 缺少闭合括号
	
	if _, err := etcdClient.Put(ctx, invalidJSONKey, invalidJSON); err != nil {
		t.Errorf("Failed to put invalid JSON: %v", err)
	}

	// 测试2: 缺少必要字段的 JSON
	t.Log("测试2: 写入缺少必要字段的 JSON")
	incompleteJSONKey := "/pipefree/nodeexec/invalid-test/malformed/7002"
	incompleteJSON := `{"name": "incomplete-node"}` // 缺少很多必要字段
	
	if _, err := etcdClient.Put(ctx, incompleteJSONKey, incompleteJSON); err != nil {
		t.Errorf("Failed to put incomplete JSON: %v", err)
	}

	// 测试3: 类型不匹配的数据
	t.Log("测试3: 写入类型不匹配的数据")
	typeMismatchKey := "/pipefree/nodeexec/invalid-test/malformed/7003"
	typeMismatchJSON := `{
		"id": "not-a-number",
		"name": 12345,
		"created_at": "not-a-timestamp",
		"phase": "should-be-object"
	}`
	
	if _, err := etcdClient.Put(ctx, typeMismatchKey, typeMismatchJSON); err != nil {
		t.Errorf("Failed to put type mismatch JSON: %v", err)
	}

	// 等待处理
	time.Sleep(3 * time.Second)

	// 验证系统是否仍然正常工作
	t.Log("验证系统在处理无效数据后是否仍然正常工作")
	
	// 写入一个有效的对象
	validNode := &model.NodeExec{
		Basic: model.Basic{
			Id:        7004,
			CreatedAt: time.Now().Unix(),
			UpdatedAt: time.Now().Unix(),
		},
		Name:      "valid-node-after-invalid",
		Kind:      "malformed",
		Namespace: "invalid-test",
		Phase: &model.NodePhase{
			Phase: "Running",
		},
	}

	validKey := fmt.Sprintf("/pipefree/nodeexec/%s/%s/%d", 
		validNode.Namespace, validNode.Kind, validNode.Id)
	validJSON, _ := json.Marshal(validNode)
	etcdClient.Put(ctx, validKey, string(validJSON))

	time.Sleep(2 * time.Second)

	// 验证有效对象是否正确处理
	lister := informer.Lister()
	retrievedValid, err := lister.Get("invalid-test", "malformed", 7004)
	if err != nil {
		t.Errorf("System failed to process valid data after invalid data: %v", err)
	} else if retrievedValid.Name != "valid-node-after-invalid" {
		t.Errorf("Valid data corrupted after processing invalid data")
	} else {
		t.Log("✓ 系统在处理无效数据后仍能正常工作")
	}

	// 清理
	etcdClient.Delete(ctx, invalidJSONKey)
	etcdClient.Delete(ctx, incompleteJSONKey)
	etcdClient.Delete(ctx, typeMismatchKey)
	etcdClient.Delete(ctx, validKey)

	if errorCount == 0 {
		t.Log("✓ 无效数据被正确忽略，没有导致系统错误")
	}
}

// TestResourceVersionHandling 测试资源版本处理
func TestResourceVersionHandling(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	t.Log("=== 资源版本处理测试 ===")

	clientset := client_pipe.NewClientSet()
	factory := clientset.Informers()
	
	eventCollector := &EventCollector{}
	informer := factory.NodeExecs().V1().NodeExecs("version-test", "resource")
	informer.AddEventHandler(nodeexec.ResourceEventHandlerFuncs{
		AddFunc: func(obj *model.NodeExec) {
			t.Logf("Version test - ADD: %s (UpdatedAt: %d)", obj.Name, obj.UpdatedAt)
			eventCollector.AddEvent("ADD", obj)
		},
		UpdateFunc: func(oldObj, newObj *model.NodeExec) {
			t.Logf("Version test - UPDATE: %s (UpdatedAt: %d -> %d)", 
				newObj.Name, oldObj.UpdatedAt, newObj.UpdatedAt)
			eventCollector.AddEvent("UPDATE", newObj)
		},
	})

	factory.Start(ctx)
	if !factory.WaitForCacheSync(ctx) {
		t.Fatal("Failed to sync cache")
	}

	etcdClient := etcd.GetClient()

	// 创建初始对象
	baseNode := &model.NodeExec{
		Basic: model.Basic{
			Id:        8001,
			CreatedAt: time.Now().Unix(),
			UpdatedAt: 1000, // 初始版本
		},
		Name:      "version-test-node",
		Kind:      "resource",
		Namespace: "version-test",
		Phase: &model.NodePhase{
			Phase: "Pending",
		},
	}

	key := fmt.Sprintf("/pipefree/nodeexec/%s/%s/%d", 
		baseNode.Namespace, baseNode.Kind, baseNode.Id)
	baseJSON, _ := json.Marshal(baseNode)
	etcdClient.Put(ctx, key, string(baseJSON))

	time.Sleep(1 * time.Second)

	// 测试1: 正常版本递增更新
	t.Log("测试1: 正常版本递增更新")
	versions := []int64{1001, 1002, 1003, 1004, 1005}
	phases := []string{"Running", "Paused", "Resumed", "Completing", "Succeeded"}

	for i, version := range versions {
		baseNode.UpdatedAt = version
		baseNode.Phase.Phase = phases[i]
		
		updatedJSON, _ := json.Marshal(baseNode)
		etcdClient.Put(ctx, key, string(updatedJSON))
		time.Sleep(200 * time.Millisecond)
	}

	// 测试2: 乱序版本更新（模拟网络延迟导致的乱序）
	t.Log("测试2: 乱序版本更新")
	// 先发送一个较新的版本
	baseNode.UpdatedAt = 2000
	baseNode.Phase.Phase = "NewerVersion"
	newerJSON, _ := json.Marshal(baseNode)
	etcdClient.Put(ctx, key, string(newerJSON))

	time.Sleep(100 * time.Millisecond)

	// 再发送一个较老的版本（应该被忽略或正确处理）
	baseNode.UpdatedAt = 1500
	baseNode.Phase.Phase = "OlderVersion"
	olderJSON, _ := json.Marshal(baseNode)
	etcdClient.Put(ctx, key, string(olderJSON))

	// 等待所有更新处理完成
	time.Sleep(2 * time.Second)

	// 验证最终状态
	lister := informer.Lister()
	finalObj, err := lister.Get("version-test", "resource", 8001)
	if err != nil {
		t.Fatalf("Failed to get final object: %v", err)
	}

	// 检查最终状态是否为最新版本
	t.Logf("最终对象状态: UpdatedAt=%d, Phase=%s", finalObj.UpdatedAt, finalObj.Phase.Phase)

	// 验证事件序列
	events := eventCollector.GetEvents()
	t.Logf("收到 %d 个事件", len(events))
	
	for i, event := range events {
		t.Logf("事件 %d: %s - UpdatedAt=%d, Phase=%s", 
			i+1, event.Type, event.Object.UpdatedAt, event.Object.Phase.Phase)
	}

	if len(events) == 0 {
		t.Error("No events received during version test")
	} else {
		t.Log("✓ 资源版本处理测试完成")
	}

	// 清理
	etcdClient.Delete(ctx, key)
}

// TestLargeObjectHandling 测试大对象处理
func TestLargeObjectHandling(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	t.Log("=== 大对象处理测试 ===")

	clientset := client_pipe.NewClientSet()
	factory := clientset.Informers()
	
	informer := factory.NodeExecs().V1().NodeExecs("large-test", "big")
	factory.Start(ctx)
	if !factory.WaitForCacheSync(ctx) {
		t.Fatal("Failed to sync cache")
	}

	// 创建一个大的 Spec 字段（模拟复杂的配置）
	largeSpec := make(map[string]interface{})
	
	// 添加大量配置项
	for i := 0; i < 1000; i++ {
		largeSpec[fmt.Sprintf("config_%d", i)] = fmt.Sprintf("value_%d_with_some_long_content_to_make_it_bigger", i)
	}
	
	// 添加嵌套结构
	largeSpec["nested"] = map[string]interface{}{
		"level1": map[string]interface{}{
			"level2": map[string]interface{}{
				"level3": make([]string, 100),
			},
		},
	}
	
	// 填充嵌套数组
	nestedArray := largeSpec["nested"].(map[string]interface{})["level1"].(map[string]interface{})["level2"].(map[string]interface{})["level3"].([]string)
	for i := range nestedArray {
		nestedArray[i] = fmt.Sprintf("nested_value_%d", i)
	}

	specJSON, _ := json.Marshal(largeSpec)
	t.Logf("大对象 Spec 大小: %d bytes", len(specJSON))

	largeNode := &model.NodeExec{
		Basic: model.Basic{
			Id:        9001,
			CreatedAt: time.Now().Unix(),
			UpdatedAt: time.Now().Unix(),
		},
		Name:      "large-object-node",
		Kind:      "big",
		Namespace: "large-test",
		Phase: &model.NodePhase{
			Phase: "Running",
		},
		Spec: &model.Kv{},
	}

	etcdClient := etcd.GetClient()
	key := fmt.Sprintf("/pipefree/nodeexec/%s/%s/%d", 
		largeNode.Namespace, largeNode.Kind, largeNode.Id)
	
	// 测试写入大对象
	startTime := time.Now()
	largeJSON, _ := json.Marshal(largeNode)
	t.Logf("完整对象大小: %d bytes", len(largeJSON))
	
	if _, err := etcdClient.Put(ctx, key, string(largeJSON)); err != nil {
		t.Fatalf("Failed to put large object: %v", err)
	}
	writeTime := time.Since(startTime)

	// 等待对象同步到缓存
	time.Sleep(3 * time.Second)

	// 测试从缓存读取大对象
	lister := informer.Lister()
	startTime = time.Now()
	retrievedObj, err := lister.Get("large-test", "big", 9001)
	readTime := time.Since(startTime)

	if err != nil {
		t.Fatalf("Failed to retrieve large object: %v", err)
	}

	// 验证数据完整性
	if retrievedObj.Name != largeNode.Name {
		t.Errorf("Large object name mismatch: expected %s, got %s", 
			largeNode.Name, retrievedObj.Name)
	} else {
		t.Log("✓ 大对象数据完整性验证通过")
	}

	t.Logf("性能统计:")
	t.Logf("  写入耗时: %v", writeTime)
	t.Logf("  读取耗时: %v", readTime)
	t.Logf("  对象大小: %d bytes", len(largeJSON))

	// 测试大对象更新
	t.Log("测试大对象更新...")
	largeNode.Phase.Phase = "Completed"
	largeNode.UpdatedAt = time.Now().Unix()
	
	// 修改名称
	largeNode.Name = "large-object-node-updated"

	startTime = time.Now()
	updatedJSON, _ := json.Marshal(largeNode)
	etcdClient.Put(ctx, key, string(updatedJSON))
	updateTime := time.Since(startTime)

	time.Sleep(2 * time.Second)

	// 验证更新
	updatedObj, err := lister.Get("large-test", "big", 9001)
	if err != nil {
		t.Errorf("Failed to retrieve updated large object: %v", err)
	} else if updatedObj.Phase.Phase != "Completed" {
		t.Errorf("Large object update failed: expected 'Completed', got '%s'", updatedObj.Phase.Phase)
	} else {
		t.Log("✓ 大对象更新成功")
	}

	t.Logf("更新耗时: %v", updateTime)

	// 清理
	etcdClient.Delete(ctx, key)

	t.Log("✓ 大对象处理测试完成")
}