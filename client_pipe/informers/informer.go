package informers

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/yxxchange/pipefree/client_pipe/cache"
	"github.com/yxxchange/pipefree/client_pipe/typed"
	"github.com/yxxchange/pipefree/helper/log"
	"github.com/yxxchange/pipefree/infra/dal/model"
)

// ResourceEventHandler 资源事件处理器
type ResourceEventHandler interface {
	OnAdd(obj *model.NodeExec)
	OnUpdate(oldObj, newObj *model.NodeExec)
	OnDelete(obj *model.NodeExec)
}

// ResourceEventHandlerFuncs 资源事件处理函数
type ResourceEventHandlerFuncs struct {
	AddFunc    func(obj *model.NodeExec)
	UpdateFunc func(oldObj, newObj *model.NodeExec)
	DeleteFunc func(obj *model.NodeExec)
}

// OnAdd 添加事件
func (r ResourceEventHandlerFuncs) OnAdd(obj *model.NodeExec) {
	if r.AddFunc != nil {
		r.AddFunc(obj)
	}
}

// OnUpdate 更新事件
func (r ResourceEventHandlerFuncs) OnUpdate(oldObj, newObj *model.NodeExec) {
	if r.UpdateFunc != nil {
		r.UpdateFunc(oldObj, newObj)
	}
}

// OnDelete 删除事件
func (r ResourceEventHandlerFuncs) OnDelete(obj *model.NodeExec) {
	if r.DeleteFunc != nil {
		r.DeleteFunc(obj)
	}
}

// SharedInformer 共享Informer接口
type SharedInformer interface {
	AddEventHandler(handler ResourceEventHandler)
	Run(ctx context.Context) error
	HasSynced() bool
	GetStore() cache.Store
}

// sharedInformer Informer实现
type sharedInformer struct {
	client    typed.NodeExecInterface
	namespace string
	kind      string

	store     cache.Store
	reflector cache.Reflector
	handlers  []ResourceEventHandler
	synced    bool
	mu        sync.RWMutex

	resyncPeriod time.Duration

	// 事件处理
	processor *eventProcessor
}

// NewSharedInformer 创建共享Informer
func NewSharedInformer(client typed.NodeExecInterface, namespace, kind string, resyncPeriod time.Duration) SharedInformer {
	store := cache.NewStore()
	reflector := cache.NewReflector(client, store, namespace, kind, resyncPeriod)

	processor := newEventProcessor()

	return &sharedInformer{
		client:       client,
		namespace:    namespace,
		kind:         kind,
		store:        store,
		reflector:    reflector,
		handlers:     make([]ResourceEventHandler, 0),
		resyncPeriod: resyncPeriod,
		processor:    processor,
	}
}

// HasSynced 是否已同步
func (s *sharedInformer) HasSynced() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.synced
}

// GetStore 获取存储
func (s *sharedInformer) GetStore() cache.Store {
	return s.store
}

// Run 运行Informer
func (s *sharedInformer) Run(ctx context.Context) error {
	log.Infof("Starting informer for namespace=%s, kind=%s", s.namespace, s.kind)

	// 启动事件处理器
	go s.processor.run(ctx)

	// 执行初始 List 操作
	if err := s.initialList(ctx); err != nil {
		return fmt.Errorf("initial list failed: %w", err)
	}

	log.Infof("Initial list completed for namespace=%s, kind=%s, starting watch...", s.namespace, s.kind)

	// 启动 Watch 循环，只有 Watch 成功建立连接后才标记为同步
	return s.watchLoop(ctx)
}

// initialList 执行初始 List 操作
func (s *sharedInformer) initialList(ctx context.Context) error {
	opts := typed.ListOptions{
		Namespace: s.namespace,
		Kind:      s.kind,
	}

	list, err := s.client.List(ctx, opts)
	if err != nil {
		return fmt.Errorf("failed to list: %w", err)
	}

	// 将所有对象添加到 store 并发送 ADD 事件
	for _, obj := range list.Items {
		key := s.getKey(obj)
		s.store.Add(key, obj)
		s.notifyAdd(obj)
	}

	log.Infof("Listed %d items for namespace=%s, kind=%s", len(list.Items), s.namespace, s.kind)
	return nil
}

// watchLoop Watch 循环
func (s *sharedInformer) watchLoop(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			log.Infof("Stopping informer for namespace=%s, kind=%s", s.namespace, s.kind)
			return ctx.Err()
		default:
		}

		// 启动 Watch
		if err := s.watch(ctx); err != nil {
			log.Errorf("Watch failed: %v", err)
			// Watch 失败后，等待一段时间再重试
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Second):
				// 重新执行 List 操作以确保数据一致性
				if listErr := s.resync(ctx); listErr != nil {
					log.Errorf("Re-list after watch failure failed: %v", listErr)
				}
			}
		}
	}
}

// watch 执行 Watch 操作
func (s *sharedInformer) watch(ctx context.Context) error {
	opts := typed.WatchOptions{
		Namespace: s.namespace,
		Kind:      s.kind,
	}

	log.Infof("🔍 Starting watch for namespace=%s, kind=%s", s.namespace, s.kind)
	watcher, err := s.client.Watch(ctx, opts)
	if err != nil {
		log.Errorf("❌ Failed to start watch: %v", err)
		return fmt.Errorf("failed to start watch: %w", err)
	}
	defer watcher.Stop()

	// 只有在 Watch 成功启动后才标记为已同步
	s.mu.Lock()
	if !s.synced {
		s.synced = true
		log.Infof("✅ Informer synced for namespace=%s, kind=%s", s.namespace, s.kind)
	}
	s.mu.Unlock()

	// 设置重新同步定时器
	resyncTicker := time.NewTicker(s.resyncPeriod)
	defer resyncTicker.Stop()

	log.Infof("👂 Watch connection established, listening for events...")

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case event, ok := <-watcher.ResultChan():
			if !ok {
				log.Warnf("🔌 Watch channel closed")
				return fmt.Errorf("watch channel closed")
			}

			if err := s.handleWatchEvent(event); err != nil {
				log.Errorf("Failed to handle watch event: %v", err)
			}

		case <-resyncTicker.C:
			// 定期重新同步
			if err := s.resync(ctx); err != nil {
				log.Errorf("Resync failed: %v", err)
			}
		}
	}
}

// handleWatchEvent 处理 Watch 事件
func (s *sharedInformer) handleWatchEvent(event typed.Event) error {
	log.Infof("📨 Informer received event: type=%s", event.Type)

	if event.Error != nil {
		log.Errorf("❌ Event error: %v", event.Error)
		return event.Error
	}

	if event.Object == nil {
		// 对于删除事件，Object 可能为空
		if event.Type != typed.EventTypeDeleted {
			log.Infof("❌ Received event with nil object: %s", event.Type)
		}
		return nil
	}

	key := s.getKey(event.Object)
	log.Infof("🔑 Event key: %s, node: %s, phase: %s",
		key, event.Object.Name, event.Object.Phase.Phase)

	switch event.Type {
	case typed.EventTypeAdded:
		s.store.Add(key, event.Object)
		log.Infof("➕ Notifying ADD event for: %s", key)
		s.notifyAdd(event.Object)

	case typed.EventTypeModified:
		oldObj, _ := s.store.Get(key)
		s.store.Update(key, event.Object)
		log.Infof("🔄 Notifying UPDATE event for: %s", key)
		s.notifyUpdate(oldObj, event.Object)

	case typed.EventTypeDeleted:
		oldObj, _ := s.store.Get(key)
		s.store.Delete(key)
		if oldObj != nil {
			log.Infof("❌ Notifying DELETE event for: %s", key)
			s.notifyDelete(oldObj)
		}

	default:
		log.Warnf("❓ Unknown event type: %s", event.Type)
	}

	return nil
}

// resync 重新同步
func (s *sharedInformer) resync(ctx context.Context) error {
	log.Debugf("Resyncing informer for namespace=%s, kind=%s", s.namespace, s.kind)

	opts := typed.ListOptions{
		Namespace: s.namespace,
		Kind:      s.kind,
	}

	list, err := s.client.List(ctx, opts)
	if err != nil {
		return fmt.Errorf("failed to list during resync: %w", err)
	}

	// 构建当前 etcd 中的对象映射
	currentKeys := make(map[string]bool)
	for _, item := range list.Items {
		key := s.getKey(item)
		currentKeys[key] = true

		oldObj, exists := s.store.Get(key)
		if !exists {
			// 新对象，添加到 store 并通知
			s.store.Add(key, item)
			s.notifyAdd(item)
		} else if !s.objectsEqual(oldObj, item) {
			// 对象已更新，更新 store 并通知
			s.store.Update(key, item)
			s.notifyUpdate(oldObj, item)
		}
	}

	// 检查 store 中是否有已被删除的对象
	storeKeys := s.store.ListKeys()
	for _, key := range storeKeys {
		if !currentKeys[key] {
			// 对象已被删除，从 store 中移除并通知
			oldObj, _ := s.store.Get(key)
			s.store.Delete(key)
			if oldObj != nil {
				s.notifyDelete(oldObj)
			}
		}
	}

	return nil
}

// AddEventHandler 添加事件处理器
func (s *sharedInformer) AddEventHandler(handler ResourceEventHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handlers = append(s.handlers, handler)

	// 如果已经同步，立即发送当前所有对象的 Add 事件
	if s.synced {
		go s.sendCurrentStateToHandler(handler)
	}
}

// sendCurrentStateToHandler 向新的处理器发送当前状态
func (s *sharedInformer) sendCurrentStateToHandler(handler ResourceEventHandler) {
	objs := s.store.List()
	for _, obj := range objs {
		handler.OnAdd(obj)
	}
}

// getKey 获取对象键
func (s *sharedInformer) getKey(obj *model.NodeExec) string {
	return fmt.Sprintf("%s/%s/%d", obj.Namespace, obj.Kind, obj.Id)
}

// eventProcessor 事件处理器
type eventProcessor struct {
	handlers []ResourceEventHandler
	mu       sync.RWMutex
}

// newEventProcessor 创建事件处理器
func newEventProcessor() *eventProcessor {
	return &eventProcessor{
		handlers: make([]ResourceEventHandler, 0),
	}
}

// run 运行事件处理器
func (p *eventProcessor) run(ctx context.Context) {
	// 这里可以实现事件队列和批处理逻辑
	// 目前保持简单实现
	<-ctx.Done()
}

// objectsEqual 比较对象是否相等
func (s *sharedInformer) objectsEqual(old, new *model.NodeExec) bool {
	return old.UpdatedAt == new.UpdatedAt
}

// 通知方法
func (s *sharedInformer) notifyAdd(obj *model.NodeExec) {
	s.mu.RLock()
	handlers := make([]ResourceEventHandler, len(s.handlers))
	copy(handlers, s.handlers)
	s.mu.RUnlock()

	log.Infof("📢 Notifying %d handlers for ADD event: %s", len(handlers), obj.Name)
	for i, handler := range handlers {
		log.Debugf("📢 Calling handler %d for ADD", i)
		handler.OnAdd(obj)
	}
}

func (s *sharedInformer) notifyUpdate(oldObj, newObj *model.NodeExec) {
	s.mu.RLock()
	handlers := make([]ResourceEventHandler, len(s.handlers))
	copy(handlers, s.handlers)
	s.mu.RUnlock()

	log.Infof("📢 Notifying %d handlers for UPDATE event: %s", len(handlers), newObj.Name)
	for i, handler := range handlers {
		log.Debugf("📢 Calling handler %d for UPDATE", i)
		handler.OnUpdate(oldObj, newObj)
	}
}

func (s *sharedInformer) notifyDelete(obj *model.NodeExec) {
	s.mu.RLock()
	handlers := make([]ResourceEventHandler, len(s.handlers))
	copy(handlers, s.handlers)
	s.mu.RUnlock()

	log.Infof("📢 Notifying %d handlers for DELETE event: %s", len(handlers), obj.Name)
	for i, handler := range handlers {
		log.Debugf("📢 Calling handler %d for DELETE", i)
		handler.OnDelete(obj)
	}
}
