package typed

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/yxxchange/pipefree/helper/log"
	"github.com/yxxchange/pipefree/infra/dal/dao"
	"github.com/yxxchange/pipefree/infra/dal/model"
	"github.com/yxxchange/pipefree/infra/etcd"
	"github.com/yxxchange/pipefree/service/pipe_exec"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// getEtcdClient 获取etcd客户端
func getEtcdClient() *clientv3.Client {
	return etcd.GetClient()
}

// EventType 事件类型
type EventType string

const (
	EventTypeAdded    EventType = "ADDED"
	EventTypeModified EventType = "MODIFIED"
	EventTypeDeleted  EventType = "DELETED"
	EventTypeError    EventType = "ERROR"
)

// Event 事件
type Event struct {
	Type   EventType       `json:"type"`
	Object *model.NodeExec `json:"object,omitempty"`
	Error  error           `json:"error,omitempty"`
}

// ListOptions 列表选项
type ListOptions struct {
	Namespace     string
	Kind          string
	LabelSelector string
	Limit         int64
	Continue      string
}

// WatchOptions 监听选项
type WatchOptions struct {
	Namespace       string
	Kind            string
	LabelSelector   string
	ResourceVersion string
	TimeoutSeconds  *int64
}

// WatchInterface 监听接口
type WatchInterface interface {
	Stop()
	ResultChan() <-chan Event
}

// NodeExecInterface 节点执行接口
type NodeExecInterface interface {
	List(ctx context.Context, opts ListOptions) (*NodeExecList, error)
	Watch(ctx context.Context, opts WatchOptions) (WatchInterface, error)
	ListAndWatch(ctx context.Context, opts ListOptions) (WatchInterface, error)
	Get(ctx context.Context, namespace, kind string, id int64) (*model.NodeExec, error)
}

// NodeExecList 节点执行列表
type NodeExecList struct {
	Items           []*model.NodeExec `json:"items"`
	ResourceVersion string            `json:"resourceVersion"`
	Continue        string            `json:"continue,omitempty"`
}

// nodeExecClient 节点执行客户端实现
type nodeExecClient struct {
	watchers sync.Map
}

// NewNodeExecClient 创建节点执行客户端
func NewNodeExecClient() NodeExecInterface {
	return &nodeExecClient{}
}

// List 列表查询
func (c *nodeExecClient) List(ctx context.Context, opts ListOptions) (*NodeExecList, error) {
	query := dao.Q.NodeExec.WithContext(ctx)
	
	if opts.Namespace != "" {
		query = query.Where(dao.NodeExec.Namespace.Eq(opts.Namespace))
	}
	if opts.Kind != "" {
		query = query.Where(dao.NodeExec.Kind.Eq(opts.Kind))
	}
	
	if opts.Limit > 0 {
		query = query.Limit(int(opts.Limit))
	}
	
	items, err := query.Find()
	if err != nil {
		return nil, fmt.Errorf("failed to list node execs: %w", err)
	}
	
	resourceVersion := strconv.FormatInt(time.Now().UnixNano(), 10)
	if len(items) > 0 {
		latest := items[0]
		for _, item := range items {
			if item.UpdatedAt > latest.UpdatedAt {
				latest = item
			}
		}
		resourceVersion = strconv.FormatInt(latest.UpdatedAt, 10)
	}
	
	return &NodeExecList{
		Items:           items,
		ResourceVersion: resourceVersion,
	}, nil
}

// Get 获取单个资源
func (c *nodeExecClient) Get(ctx context.Context, namespace, kind string, id int64) (*model.NodeExec, error) {
	query := dao.Q.NodeExec.WithContext(ctx).Where(dao.NodeExec.Id.Eq(id))
	
	if namespace != "" {
		query = query.Where(dao.NodeExec.Namespace.Eq(namespace))
	}
	if kind != "" {
		query = query.Where(dao.NodeExec.Kind.Eq(kind))
	}
	
	item, err := query.First()
	if err != nil {
		return nil, fmt.Errorf("failed to get node exec: %w", err)
	}
	
	return item, nil
}

// Watch 监听资源变化
func (c *nodeExecClient) Watch(ctx context.Context, opts WatchOptions) (WatchInterface, error) {
	keyPrefix := fmt.Sprintf(pipe_exec.KeyPrefixTemplate, opts.Namespace, opts.Kind)
	
	w := &watcher{
		keyPrefix:       keyPrefix,
		namespace:       opts.Namespace,
		kind:           opts.Kind,
		resourceVersion: opts.ResourceVersion,
		resultChan:     make(chan Event, 100),
		stopChan:       make(chan struct{}),
		client:         c,
	}
	
	watcherID := fmt.Sprintf("%s-%d", keyPrefix, time.Now().UnixNano())
	c.watchers.Store(watcherID, w)
	
	go w.start(ctx, watcherID)
	
	return w, nil
}

// ListAndWatch 先List再 Watch，避免遗漏
func (c *nodeExecClient) ListAndWatch(ctx context.Context, opts ListOptions) (WatchInterface, error) {
	// 先获取当前所有资源
	list, err := c.List(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("initial list failed: %w", err)
	}
	
	// 创建特殊的watcher，先发送List结果，再监听Watch
	keyPrefix := fmt.Sprintf(pipe_exec.KeyPrefixTemplate, opts.Namespace, opts.Kind)
	
	w := &listAndWatcher{
		keyPrefix:       keyPrefix,
		namespace:       opts.Namespace,
		kind:           opts.Kind,
		resourceVersion: list.ResourceVersion,
		resultChan:     make(chan Event, 100),
		stopChan:       make(chan struct{}),
		client:         c,
		initialList:    list.Items,
	}
	
	watcherID := fmt.Sprintf("listwatch-%s-%d", keyPrefix, time.Now().UnixNano())
	c.watchers.Store(watcherID, w)
	
	go w.start(ctx, watcherID)
	
	return w, nil
}

// watcher 监听器实现
type watcher struct {
	keyPrefix       string
	namespace       string
	kind           string
	resourceVersion string
	resultChan     chan Event
	stopChan       chan struct{}
	client         *nodeExecClient
	stopped        bool
	mu             sync.RWMutex
}

// ResultChan 获取结果通道
func (w *watcher) ResultChan() <-chan Event {
	return w.resultChan
}

// Stop 停止监听
func (w *watcher) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()
	
	if w.stopped {
		return
	}
	
	w.stopped = true
	close(w.stopChan)
	close(w.resultChan)
}

// start 启动监听
func (w *watcher) start(ctx context.Context, watcherID string) {
	defer func() {
		w.client.watchers.Delete(watcherID)
		if r := recover(); r != nil {
			log.Errorf("watcher panic: %v", r)
			w.sendEvent(Event{
				Type:  EventTypeError,
				Error: fmt.Errorf("watcher panic: %v", r),
			})
		}
	}()
	
	if err := w.initialList(ctx); err != nil {
		log.Errorf("initial list failed: %v", err)
		w.sendEvent(Event{
			Type:  EventTypeError,
			Error: fmt.Errorf("initial list failed: %w", err),
		})
		return
	}
	
	w.watchEtcd(ctx)
}

// initialList 初始列表同步
func (w *watcher) initialList(ctx context.Context) error {
	opts := ListOptions{
		Namespace: w.namespace,
		Kind:     w.kind,
	}
	
	list, err := w.client.List(ctx, opts)
	if err != nil {
		return err
	}
	
	for _, item := range list.Items {
		select {
		case <-w.stopChan:
			return nil
		default:
			w.sendEvent(Event{
				Type:   EventTypeAdded,
				Object: item,
			})
		}
	}
	
	w.resourceVersion = list.ResourceVersion
	return nil
}

// watchEtcd 监听etcd变化
func (w *watcher) watchEtcd(ctx context.Context) {
	// 使用全局etcd客户端
	etcdClient := getEtcdClient()
	
	watchCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	
	watchChan := etcdClient.Watch(watchCtx, w.keyPrefix, clientv3.WithPrefix())
	
	for {
		select {
		case <-w.stopChan:
			return
		case <-ctx.Done():
			return
		case watchResp, ok := <-watchChan:
			if !ok {
				log.Warnf("watch channel closed, restarting watch for %s", w.keyPrefix)
				time.Sleep(time.Second)
				watchChan = etcdClient.Watch(watchCtx, w.keyPrefix, clientv3.WithPrefix())
				continue
			}
			
			if watchResp.Err() != nil {
				log.Errorf("watch error: %v", watchResp.Err())
				w.sendEvent(Event{
					Type:  EventTypeError,
					Error: watchResp.Err(),
				})
				continue
			}
			
			for _, event := range watchResp.Events {
				if err := w.handleEtcdEvent(ctx, event); err != nil {
					log.Errorf("handle etcd event failed: %v", err)
					w.sendEvent(Event{
						Type:  EventTypeError,
						Error: err,
					})
				}
			}
		}
	}
}

// handleEtcdEvent 处理etcd事件
func (w *watcher) handleEtcdEvent(ctx context.Context, event *clientv3.Event) error {
	var nodeExec model.NodeExec
	
	switch event.Type {
	case clientv3.EventTypePut:
		if err := json.Unmarshal(event.Kv.Value, &nodeExec); err != nil {
			return fmt.Errorf("failed to unmarshal node exec: %w", err)
		}
		
		if w.namespace != "" && nodeExec.Namespace != w.namespace {
			return nil
		}
		if w.kind != "" && nodeExec.Kind != w.kind {
			return nil
		}
		
		eventType := EventTypeAdded
		if event.Kv.Version > 1 {
			eventType = EventTypeModified
		}
		
		w.sendEvent(Event{
			Type:   eventType,
			Object: &nodeExec,
		})
		
	case clientv3.EventTypeDelete:
		w.sendEvent(Event{
			Type: EventTypeDeleted,
		})
	}
	
	return nil
}

// sendEvent 发送事件
func (w *watcher) sendEvent(event Event) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	
	if w.stopped {
		return
	}
	
	select {
	case w.resultChan <- event:
	case <-time.After(5 * time.Second):
		log.Warnf("send event timeout, dropping event: %+v", event)
	}
}

// listAndWatcher ListAndWatch的特殊实现
type listAndWatcher struct {
	keyPrefix       string
	namespace       string
	kind           string
	resourceVersion string
	resultChan     chan Event
	stopChan       chan struct{}
	client         *nodeExecClient
	initialList    []*model.NodeExec
	stopped        bool
	mu             sync.RWMutex
}

// ResultChan 获取结果通道
func (w *listAndWatcher) ResultChan() <-chan Event {
	return w.resultChan
}

// Stop 停止监听
func (w *listAndWatcher) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()
	
	if w.stopped {
		return
	}
	
	w.stopped = true
	close(w.stopChan)
	close(w.resultChan)
}

// start 启动ListAndWatch
func (w *listAndWatcher) start(ctx context.Context, watcherID string) {
	defer func() {
		w.client.watchers.Delete(watcherID)
		if r := recover(); r != nil {
			log.Errorf("listAndWatcher panic: %v", r)
			w.sendEvent(Event{
				Type:  EventTypeError,
				Error: fmt.Errorf("listAndWatcher panic: %v", r),
			})
		}
	}()
	
	// 先发送初始列表中的所有资源
	for _, item := range w.initialList {
		select {
		case <-w.stopChan:
			return
		default:
			w.sendEvent(Event{
				Type:   EventTypeAdded,
				Object: item,
			})
		}
	}
	
	// 然后开始监听etcd变化
	w.watchEtcd(ctx)
}

// watchEtcd 监听etcd变化
func (w *listAndWatcher) watchEtcd(ctx context.Context) {
	etcdClient := getEtcdClient()
	
	watchCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	
	watchChan := etcdClient.Watch(watchCtx, w.keyPrefix, clientv3.WithPrefix())
	
	for {
		select {
		case <-w.stopChan:
			return
		case <-ctx.Done():
			return
		case watchResp, ok := <-watchChan:
			if !ok {
				log.Warnf("watch channel closed, restarting watch for %s", w.keyPrefix)
				time.Sleep(time.Second)
				watchChan = etcdClient.Watch(watchCtx, w.keyPrefix, clientv3.WithPrefix())
				continue
			}
			
			if watchResp.Err() != nil {
				log.Errorf("watch error: %v", watchResp.Err())
				w.sendEvent(Event{
					Type:  EventTypeError,
					Error: watchResp.Err(),
				})
				continue
			}
			
			for _, event := range watchResp.Events {
				if err := w.handleEtcdEvent(ctx, event); err != nil {
					log.Errorf("handle etcd event failed: %v", err)
					w.sendEvent(Event{
						Type:  EventTypeError,
						Error: err,
					})
				}
			}
		}
	}
}

// handleEtcdEvent 处理etcd事件
func (w *listAndWatcher) handleEtcdEvent(ctx context.Context, event *clientv3.Event) error {
	var nodeExec model.NodeExec
	
	switch event.Type {
	case clientv3.EventTypePut:
		if err := json.Unmarshal(event.Kv.Value, &nodeExec); err != nil {
			return fmt.Errorf("failed to unmarshal node exec: %w", err)
		}
		
		if w.namespace != "" && nodeExec.Namespace != w.namespace {
			return nil
		}
		if w.kind != "" && nodeExec.Kind != w.kind {
			return nil
		}
		
		// 对于ListAndWatch，所有PUT事件都作为Modified处理
		// 因为初始的Added事件已经在List阶段发送过了
		w.sendEvent(Event{
			Type:   EventTypeModified,
			Object: &nodeExec,
		})
		
	case clientv3.EventTypeDelete:
		w.sendEvent(Event{
			Type: EventTypeDeleted,
		})
	}
	
	return nil
}

// sendEvent 发送事件
func (w *listAndWatcher) sendEvent(event Event) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	
	if w.stopped {
		return
	}
	
	select {
	case w.resultChan <- event:
	case <-time.After(5 * time.Second):
		log.Warnf("send event timeout, dropping event: %+v", event)
	}
}