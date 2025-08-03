# PipeFree 集成测试

## 测试目标
验证客户端与服务端同时工作时的：
1. **完整性**：客户端监听不重不漏
2. **最终一致性**：数据库、etcd、客户端缓存三方一致

## 可用测试

### `correct_test.go` - 集成测试
```bash
go test -v -run TestCorrectIntegration
```
- ✅ 客户端和服务端同时工作
- 📊 验证完整性和一致性
- 🎯 使用ListAndWatch避免遗漏

## 核心改进

### ListAndWatch模式
参考client-go的最佳实践，实现了`ListAndWatch`方法：

1. **先List**：获取当前所有存在的资源
2. **再Watch**：监听增量变化
3. **无缝衔接**：使用List的ResourceVersion作为Watch起点

```go
// 使用ListAndWatch替代单独的Watch
watcher, err := client.ListAndWatch(ctx, typed.ListOptions{
    Namespace: "test",
    Kind:      "task",
})
```

### 避免遗漏的机制
- **初始同步**：List阶段发送所有现有资源的ADDED事件
- **增量更新**：Watch阶段只处理MODIFIED/DELETED事件
- **版本控制**：使用ResourceVersion确保连续性

## 测试流程

### 服务端模拟：
1. 创建3个节点（build, test, deploy）
2. 每个节点状态变化：Ready → Pending → Running → Succeeded
3. 同时更新MySQL和etcd

### 客户端验证：
1. ListAndWatch实时监听
2. 记录所有接收到的事件
3. 查询最终状态

### 验证指标：
- **完整性**：客户端事件数 ≥ 服务端操作数
- **一致性**：最终状态所有节点都是Succeeded

## 运行要求
- MySQL已启动
- etcd已启动  
- 配置文件 `../config.yaml` 存在