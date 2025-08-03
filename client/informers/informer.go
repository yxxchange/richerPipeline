package informers

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/yxxchange/pipefree/client/cache"
	"github.com/yxxchange/pipefree/client/typed"
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
	client     typed.NodeExecInterface
	namespace  string
	kind       string
	
	store      cache.Store
	handlers   []ResourceEventHandler
	synced     bool
	mu         sync.RWMutex
	
	resyncPeriod time.Duration
}

// NewSharedInformer 创建共享Informer
func NewSharedInformer(client typed.NodeExecInterface, namespace, kind string, resyncPeriod time.Duration) SharedInformer {
	return &sharedInformer{
		client:       client,
		namespace:    namespace,
		kind:        kind,
		store:       cache.NewStore(),
		handlers:    make([]ResourceEventHandler, 0),
		resyncPeriod: resyncPeriod,
	}
}

// AddEventHandler 添加事件处理器
func (s *sharedInformer) AddEventHandler(handler ResourceEventHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handlers = append(s.handlers, handler)
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
	
	watchOpts := typed.WatchOptions{
		Namespace: s.namespace,
		Kind:     s.kind,
	}
	
	watcher, err := s.client.Watch(ctx, watchOpts)
	if err != nil {
		return fmt.Errorf("failed to start watch: %w", err)
	}
	defer watcher.Stop()
	
	resyncTicker := time.NewTicker(s.resyncPeriod)
	defer resyncTicker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			log.Infof("Stopping informer for namespace=%s, kind=%s", s.namespace, s.kind)
			return ctx.Err()
			
		case event, ok := <-watcher.ResultChan():
			if !ok {
				log.Warnf("Watch channel closed, restarting...")
				return fmt.Errorf("watch channel closed")
			}
			
			if err := s.handleEvent(event); err != nil {
				log.Errorf("Failed to handle event: %v", err)
			}
			
		case <-resyncTicker.C:
			if err := s.resync(ctx); err != nil {
				log.Errorf("Failed to resync: %v", err)
			}
		}
	}
}

// handleEvent 处理事件
func (s *sharedInformer) handleEvent(event typed.Event) error {
	if event.Error != nil {
		return event.Error
	}
	
	if event.Object == nil {
		return nil
	}
	
	key := s.getKey(event.Object)
	
	switch event.Type {
	case typed.EventTypeAdded:
		s.store.Add(key, event.Object)
		s.notifyAdd(event.Object)
		
		if !s.synced {
			s.mu.Lock()
			s.synced = true
			s.mu.Unlock()
			log.Infof("Informer synced for namespace=%s, kind=%s", s.namespace, s.kind)
		}
		
	case typed.EventTypeModified:
		oldObj, _ := s.store.Get(key)
		s.store.Update(key, event.Object)
		s.notifyUpdate(oldObj, event.Object)
		
	case typed.EventTypeDeleted:
		oldObj, _ := s.store.Get(key)
		s.store.Delete(key)
		if oldObj != nil {
			s.notifyDelete(oldObj)
		}
	}
	
	return nil
}

// resync 重新同步
func (s *sharedInformer) resync(ctx context.Context) error {
	log.Debugf("Resyncing informer for namespace=%s, kind=%s", s.namespace, s.kind)
	
	opts := typed.ListOptions{
		Namespace: s.namespace,
		Kind:     s.kind,
	}
	
	list, err := s.client.List(ctx, opts)
	if err != nil {
		return fmt.Errorf("failed to list during resync: %w", err)
	}
	
	currentKeys := make(map[string]bool)
	for _, item := range list.Items {
		key := s.getKey(item)
		currentKeys[key] = true
		
		oldObj, exists := s.store.Get(key)
		if !exists {
			s.store.Add(key, item)
			s.notifyAdd(item)
		} else if !s.objectsEqual(oldObj, item) {
			s.store.Update(key, item)
			s.notifyUpdate(oldObj, item)
		}
	}
	
	storeKeys := s.store.ListKeys()
	for _, key := range storeKeys {
		if !currentKeys[key] {
			oldObj, _ := s.store.Get(key)
			s.store.Delete(key)
			if oldObj != nil {
				s.notifyDelete(oldObj)
			}
		}
	}
	
	return nil
}

// getKey 获取对象键
func (s *sharedInformer) getKey(obj *model.NodeExec) string {
	return fmt.Sprintf("%s/%s/%d", obj.Namespace, obj.Kind, obj.Id)
}

// objectsEqual 比较对象是否相等
func (s *sharedInformer) objectsEqual(old, new *model.NodeExec) bool {
	return old.UpdatedAt.Equal(new.UpdatedAt)
}

// 通知方法
func (s *sharedInformer) notifyAdd(obj *model.NodeExec) {
	s.mu.RLock()
	handlers := make([]ResourceEventHandler, len(s.handlers))
	copy(handlers, s.handlers)
	s.mu.RUnlock()
	
	for _, handler := range handlers {
		handler.OnAdd(obj)
	}
}

func (s *sharedInformer) notifyUpdate(oldObj, newObj *model.NodeExec) {
	s.mu.RLock()
	handlers := make([]ResourceEventHandler, len(s.handlers))
	copy(handlers, s.handlers)
	s.mu.RUnlock()
	
	for _, handler := range handlers {
		handler.OnUpdate(oldObj, newObj)
	}
}

func (s *sharedInformer) notifyDelete(obj *model.NodeExec) {
	s.mu.RLock()
	handlers := make([]ResourceEventHandler, len(s.handlers))
	copy(handlers, s.handlers)
	s.mu.RUnlock()
	
	for _, handler := range handlers {
		handler.OnDelete(obj)
	}
}