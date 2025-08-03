# PipeFree Client

类似于 Kubernetes client-go 的声明式 API 客户端，提供 List 和 Watch 功能，保证数据最终一致性。

## 目录结构

```
client/
├── clientset.go              # 主客户端入口
├── typed/                    # 类型化客户端
│   └── nodeexec/
│       └── v1/
│           ├── nodeexec_client.go  # NodeExec客户端实现
│           └── types.go            # 类型定义
├── informers/                # Informer实现
│   ├── factory.go           # Informer工厂
│   ├── informer.go          # 核心Informer实现
│   └── nodeexec/
│       └── interface.go     # NodeExec Informer接口
├── listers/                 # Lister实现
│   └── nodeexec/
│       └── v1/
│           └── nodeexec.go  # NodeExec Lister
├── cache/                   # 缓存实现
│   └── store.go            # 本地存储
├── util/                    # 工具类
│   └── workqueue.go        # 工作队列
└── example/                 # 使用示例
    └── main.go
```

## 核心特性

- **声明式 API**: 类似 K8s 的资源管理方式
- **List & Watch**: 支持列表查询和实时监听
- **最终一致性**: 通过 etcd 保证分布式一致性
- **不重不漏**: 基于 etcd watch 机制，确保事件不丢失
- **本地缓存**: Informer 模式提供高性能本地缓存
- **事件处理**: 支持自定义事件处理器
- **分层架构**: 清晰的目录结构，易于扩展

## 架构设计

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Application   │    │     Client      │    │      etcd       │
│                 │    │                 │    │                 │
│  ┌───────────┐  │    │  ┌───────────┐  │    │  ┌───────────┐  │
│  │ Informer  │◄─┼────┼─►│   Watch   │◄─┼────┼─►│   Watch   │  │
│  └───────────┘  │    │  └───────────┘  │    │  └───────────┘  │
│  ┌───────────┐  │    │  ┌───────────┐  │    │  ┌───────────┐  │
│  │  Lister   │◄─┼────┼─►│   List    │◄─┼────┼─►│    KV     │  │
│  └───────────┘  │    │  └───────────┘  │    │  └───────────┘  │
│  ┌───────────┐  │    │  ┌───────────┐  │    │                 │
│  │   Store   │  │    │  │   Cache   │  │    │                 │
│  └───────────┘  │    │  └───────────┘  │    │                 │
└─────────────────┘    └─────────────────┘    └─────────────────┘
```

## 使用方式

### 1. 直接使用 Typed Client

```go
// 创建客户端
clientset := client.NewForConfig()

// List 操作
listOpts := v1.ListOptions{
    Namespace: "default",
    Kind:      "task",
}
list, err := clientset.NodeExecsV1().List(ctx, listOpts)

// Watch 操作
watchOpts := v1.WatchOptions{
    Namespace: "default", 
    Kind:      "task",
}
watcher, err := clientset.NodeExecsV1().Watch(ctx, watchOpts)
defer watcher.Stop()

for event := range watcher.ResultChan() {
    switch event.Type {
    case v1.EventTypeAdded:
        // 处理新增事件
    case v1.EventTypeModified:
        // 处理修改事件
    case v1.EventTypeDeleted:
        // 处理删除事件
    }
}
```

### 2. 使用 Informer 模式（推荐）

```go
// 创建 Informer 工厂
factory := informers.NewSharedInformerFactory(clientset.NodeExecsV1(), 30*time.Second)

// 获取 NodeExec Informer
nodeExecInformer := factory.NodeExecs().V1().NodeExecs()

// 添加事件处理器
nodeExecInformer.Informer().AddEventHandler(informers.ResourceEventHandlerFuncs{
    AddFunc: func(obj *model.NodeExec) {
        // 处理新增
    },
    UpdateFunc: func(oldObj, newObj *model.NodeExec) {
        // 处理更新
    },
    DeleteFunc: func(obj *model.NodeExec) {
        // 处理删除
    },
})

// 启动 Informer
factory.Start(ctx)

// 等待缓存同步
factory.WaitForCacheSync(ctx)

// 使用 Lister 查询本地缓存（高性能）
lister := nodeExecInformer.Lister()
objects, err := lister.List("default", "task")
```

## 核心组件

### Clientset
- **Interface**: 主客户端接口
- **NodeExecsV1()**: 获取v1版本的节点执行接口

### Typed Client
- **NodeExecInterface**: 节点执行接口
- **List()**: 列表查询
- **Watch()**: 实时监听
- **Get()**: 获取单个资源

### Informer
- **SharedInformer**: 共享 Informer，避免重复监听
- **Factory**: 管理多个 Informer 实例
- **EventHandler**: 事件处理器

### Cache & Store
- **Store**: 本地缓存存储
- **Lister**: 高性能本地查询

### Util
- **WorkQueue**: 工作队列
- **DelayingWorkQueue**: 延迟工作队列

## 数据一致性保证

1. **初始同步**: Watch 启动前先执行 List 操作
2. **增量更新**: 基于 etcd watch 机制获取增量变化
3. **定期重同步**: 定期执行 List 操作确保数据一致性
4. **版本控制**: 使用 ResourceVersion 避免重复处理
5. **错误重试**: 监听中断时自动重新连接

## 性能优化

- **本地缓存**: Informer 提供内存缓存，避免频繁查询
- **事件去重**: 基于版本号避免重复事件
- **批量处理**: 支持批量事件处理
- **连接复用**: 复用 etcd 连接减少开销
- **分层架构**: 清晰的职责分离，便于维护和扩展

## 运行示例

```bash
cd client/example
go run main.go
```

## 扩展指南

### 添加新的资源类型

1. 在 `typed/` 下创建新的资源目录
2. 实现对应的 Client 接口
3. 在 `informers/` 下添加对应的 Informer
4. 在 `listers/` 下添加对应的 Lister
5. 更新主 Clientset 接口