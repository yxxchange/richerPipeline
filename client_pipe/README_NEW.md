# PipeFree Client

基于 Kubernetes client-go 设计理念的流水线引擎客户端库。

## 架构设计

### 核心组件

1. **Typed Client** - 直接与 etcd 交互的类型化客户端
2. **Informer** - 基于 List/Watch 机制的事件驱动缓存
3. **Reflector** - 负责从 etcd 同步数据到本地缓存
4. **Store** - 本地内存缓存，支持索引查询
5. **Lister** - 从本地缓存快速查询的接口

### 数据流

```
etcd → Reflector → Store → Lister
  ↓
Watch Events → Informer → Event Handlers
```

## 使用方式

### 1. 基本用法

```go
// 创建客户端
clientset := client.NewClientset()

// 直接使用 typed client
nodeExecClient := clientset.NodeExecs()
list, err := nodeExecClient.List(ctx, typed.ListOptions{
    Namespace: "default",
    Kind:     "pipeline",
})
```

### 2. 使用 Informer 监听事件

```go
// 获取 Informer
factory := clientset.Informers()
informer := factory.NodeExecs().V1().NodeExecs("default", "pipeline")

// 添加事件处理器
informer.AddEventHandler(informers.ResourceEventHandlerFuncs{
    AddFunc: func(obj *model.NodeExec) {
        fmt.Printf("Added: %s\n", obj.Name)
    },
    UpdateFunc: func(oldObj, newObj *model.NodeExec) {
        fmt.Printf("Updated: %s\n", newObj.Name)
    },
    DeleteFunc: func(obj *model.NodeExec) {
        fmt.Printf("Deleted: %s\n", obj.Name)
    },
})

// 启动 Informer
factory.Start(ctx)
factory.WaitForCacheSync(ctx)
```

### 3. 使用 Lister 查询缓存

```go
// 从本地缓存快速查询
lister := informer.Lister()
objects, err := lister.List("default", "pipeline")
obj, err := lister.Get("default", "pipeline", 123)
```

## 关键特性

### 1. 真正的 List/Watch 机制
- List 操作从 etcd 读取数据，不是从数据库
- Watch 监听 etcd 变化，实现实时事件通知
- 支持 ResourceVersion 避免事件丢失

### 2. 高效的本地缓存
- 内存中维护对象索引
- 支持按 namespace 和 kind 快速查询
- 自动处理缓存一致性

### 3. 事件驱动架构
- 基于事件的异步处理
- 支持多个事件处理器
- 自动重连和错误恢复

### 4. 可插拔设计
- 模块化组件设计
- 支持自定义配置
- 易于扩展和测试

## 配置选项

```go
config := client.ClientConfig{
    DefaultResyncPeriod: 30 * time.Second, // 重新同步间隔
}
clientset := client.NewClientsetWithConfig(config)
```

## 最佳实践

1. **使用 Informer 而不是轮询** - Informer 提供实时事件通知，比定期轮询更高效
2. **使用 Lister 查询缓存** - 从本地缓存查询比直接调用 API 快得多
3. **合理设置重新同步间隔** - 根据数据变化频率调整 ResyncPeriod
4. **处理事件处理器中的错误** - 确保事件处理器不会 panic 影响整个 Informer

## 与 Kubernetes client-go 的对比

| 特性 | client-go | pipefree-client |
|------|-----------|-----------------|
| 数据源 | Kubernetes API Server | etcd |
| 缓存机制 | Informer + Store | Informer + Store |
| 事件处理 | ResourceEventHandler | ResourceEventHandler |
| 查询接口 | Lister | Lister |
| 配置方式 | RestConfig | ClientConfig |

## 示例代码

参考 `client/example/advanced_main.go` 获取完整的使用示例。