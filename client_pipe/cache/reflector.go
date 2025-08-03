package cache

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/yxxchange/pipefree/client_pipe/typed"
	"github.com/yxxchange/pipefree/helper/log"
	"github.com/yxxchange/pipefree/infra/dal/model"
)

// Reflector 反射器接口
type Reflector interface {
	Run(ctx context.Context) error
	LastSyncResourceVersion() string
}

// reflector 反射器实现
type reflector struct {
	client       typed.NodeExecInterface
	store        Store
	namespace    string
	kind         string
	resyncPeriod time.Duration

	lastSyncResourceVersion string
	mu                      sync.RWMutex
}

// NewReflector 创建反射器
func NewReflector(client typed.NodeExecInterface, store Store, namespace, kind string, resyncPeriod time.Duration) Reflector {
	return &reflector{
		client:       client,
		store:        store,
		namespace:    namespace,
		kind:         kind,
		resyncPeriod: resyncPeriod,
	}
}

// Run 运行反射器
func (r *reflector) Run(ctx context.Context) error {
	log.Infof("Starting reflector for namespace=%s, kind=%s", r.namespace, r.kind)

	// 首先执行一次完整的 List 操作
	if err := r.listAndSync(ctx); err != nil {
		log.Errorf("Initial list failed: %v", err)
		return fmt.Errorf("initial list failed: %w", err)
	}

	// 启动 Watch 循环
	return r.watchLoop(ctx)
}

// LastSyncResourceVersion 获取最后同步的资源版本
func (r *reflector) LastSyncResourceVersion() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.lastSyncResourceVersion
}

// listAndSync 执行 List 操作并同步到 store
func (r *reflector) listAndSync(ctx context.Context) error {
	opts := typed.ListOptions{
		Namespace: r.namespace,
		Kind:      r.kind,
	}

	list, err := r.client.List(ctx, opts)
	if err != nil {
		return fmt.Errorf("failed to list: %w", err)
	}

	// 构建新的 items map
	items := make(map[string]*model.NodeExec)
	for _, obj := range list.Items {
		key := r.getKey(obj)
		items[key] = obj
	}

	// 替换 store 中的所有数据
	r.store.Replace(items)

	// 更新资源版本
	r.mu.Lock()
	r.lastSyncResourceVersion = list.ResourceVersion
	r.mu.Unlock()

	log.Infof("Listed %d items for namespace=%s, kind=%s, resourceVersion=%s",
		len(list.Items), r.namespace, r.kind, list.ResourceVersion)

	return nil
}

// watchLoop Watch 循环
func (r *reflector) watchLoop(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// 启动 Watch
		if err := r.watch(ctx); err != nil {
			log.Errorf("Watch failed: %v", err)

			// Watch 失败后，等待一段时间再重试
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Second):
				// 重新执行 List 操作以确保数据一致性
				if listErr := r.listAndSync(ctx); listErr != nil {
					log.Errorf("Re-list after watch failure failed: %v", listErr)
				}
			}
		}
	}
}

// watch 执行 Watch 操作
func (r *reflector) watch(ctx context.Context) error {
	opts := typed.WatchOptions{
		Namespace:       r.namespace,
		Kind:            r.kind,
		ResourceVersion: r.LastSyncResourceVersion(),
	}

	watcher, err := r.client.Watch(ctx, opts)
	if err != nil {
		return fmt.Errorf("failed to start watch: %w", err)
	}
	defer watcher.Stop()

	// 设置重新同步定时器
	resyncTicker := time.NewTicker(r.resyncPeriod)
	defer resyncTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case event, ok := <-watcher.ResultChan():
			if !ok {
				return fmt.Errorf("watch channel closed")
			}

			if err := r.handleWatchEvent(event); err != nil {
				log.Errorf("Failed to handle watch event: %v", err)
				return err
			}

		case <-resyncTicker.C:
			// 定期重新同步
			if err := r.listAndSync(ctx); err != nil {
				log.Errorf("Resync failed: %v", err)
				return err
			}
		}
	}
}

// handleWatchEvent 处理 Watch 事件
func (r *reflector) handleWatchEvent(event typed.Event) error {
	if event.Error != nil {
		return event.Error
	}

	if event.Object == nil {
		// 对于删除事件，Object 可能为空
		if event.Type != typed.EventTypeDeleted {
			return fmt.Errorf("received event with nil object: %s", event.Type)
		}
		return nil
	}

	key := r.getKey(event.Object)

	switch event.Type {
	case typed.EventTypeAdded:
		r.store.Add(key, event.Object)
		log.Debugf("Added object: %s", key)

	case typed.EventTypeModified:
		r.store.Update(key, event.Object)
		log.Debugf("Updated object: %s", key)

	case typed.EventTypeDeleted:
		r.store.Delete(key)
		log.Debugf("Deleted object: %s", key)

	default:
		log.Warnf("Unknown event type: %s", event.Type)
	}

	return nil
}

// getKey 获取对象键
func (r *reflector) getKey(obj *model.NodeExec) string {
	return fmt.Sprintf("%s/%s/%d", obj.Namespace, obj.Kind, obj.Id)
}
