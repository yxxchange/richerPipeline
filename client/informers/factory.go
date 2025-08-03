package informers

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/yxxchange/pipefree/client/informers/nodeexec"
	"github.com/yxxchange/pipefree/client/typed"
	"github.com/yxxchange/pipefree/helper/log"
)

// SharedInformerFactory 共享Informer工厂
type SharedInformerFactory interface {
	NodeExecs() nodeexec.Interface
	Start(ctx context.Context)
	WaitForCacheSync(ctx context.Context) bool
	GetInformer(namespace, kind string) SharedInformer
}

// sharedInformerFactory 工厂实现
type sharedInformerFactory struct {
	client        typed.NodeExecInterface
	defaultResync time.Duration
	
	informers        map[string]SharedInformer
	startedInformers map[string]bool
	mu               sync.Mutex
}

// NewSharedInformerFactory 创建共享Informer工厂
func NewSharedInformerFactory(client typed.NodeExecInterface, defaultResync time.Duration) SharedInformerFactory {
	return &sharedInformerFactory{
		client:           client,
		defaultResync:    defaultResync,
		informers:       make(map[string]SharedInformer),
		startedInformers: make(map[string]bool),
	}
}

// NodeExecs 获取节点执行Informer
func (f *sharedInformerFactory) NodeExecs() nodeexec.Interface {
	return nodeexec.New(f)
}

// Start 启动所有Informer
func (f *sharedInformerFactory) Start(ctx context.Context) {
	f.mu.Lock()
	defer f.mu.Unlock()
	
	for key, informer := range f.informers {
		if !f.startedInformers[key] {
			go func(key string, informer SharedInformer) {
				log.Infof("Starting informer: %s", key)
				if err := informer.Run(ctx); err != nil {
					log.Errorf("Informer %s stopped with error: %v", key, err)
				}
			}(key, informer)
			f.startedInformers[key] = true
		}
	}
}

// WaitForCacheSync 等待缓存同步
func (f *sharedInformerFactory) WaitForCacheSync(ctx context.Context) bool {
	f.mu.Lock()
	informers := make(map[string]SharedInformer)
	for k, v := range f.informers {
		informers[k] = v
	}
	f.mu.Unlock()
	
	for key, informer := range informers {
		if !f.waitForCacheSync(ctx, key, informer) {
			return false
		}
	}
	
	return true
}

// waitForCacheSync 等待单个Informer缓存同步
func (f *sharedInformerFactory) waitForCacheSync(ctx context.Context, key string, informer SharedInformer) bool {
	log.Infof("Waiting for cache sync: %s", key)
	
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			log.Warnf("Context cancelled while waiting for cache sync: %s", key)
			return false
		case <-ticker.C:
			if informer.HasSynced() {
				log.Infof("Cache synced: %s", key)
				return true
			}
		}
	}
}

// GetInformer 获取或创建Informer
func (f *sharedInformerFactory) GetInformer(namespace, kind string) SharedInformer {
	f.mu.Lock()
	defer f.mu.Unlock()
	
	key := f.getKey(namespace, kind)
	informer, exists := f.informers[key]
	if exists {
		return informer
	}
	
	informer = NewSharedInformer(f.client, namespace, kind, f.defaultResync)
	f.informers[key] = informer
	return informer
}

// getKey 生成键
func (f *sharedInformerFactory) getKey(namespace, kind string) string {
	return fmt.Sprintf("%s/%s", namespace, kind)
}