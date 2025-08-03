package typed

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/yxxchange/pipefree/helper/log"
	"github.com/yxxchange/pipefree/infra/dal/model"
)

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
	Get(ctx context.Context, namespace, kind string, id int64) (*model.NodeExec, error)
	Update(ctx context.Context, nodeExec *model.NodeExec) (*model.NodeExec, error)
	Delete(ctx context.Context, namespace, kind string, id int64) error
}

// NodeExecList 节点执行列表
type NodeExecList struct {
	Items           []*model.NodeExec `json:"items"`
	ResourceVersion string            `json:"resourceVersion"`
	Continue        string            `json:"continue,omitempty"`
}

// nodeExecClient 节点执行客户端实现
type nodeExecClient struct {
	watchers   sync.Map
	baseURL    string
	httpClient *http.Client
}

// ClientConfig 客户端配置
type ClientConfig struct {
	ServerURL string
	Timeout   time.Duration
}

// NewNodeExecClient 创建节点执行客户端
func NewNodeExecClient() NodeExecInterface {
	return &nodeExecClient{
		baseURL: "http://localhost:8080/api/v1",
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// NewNodeExecClientWithConfig 使用配置创建节点执行客户端
func NewNodeExecClientWithConfig(config ClientConfig) NodeExecInterface {
	return &nodeExecClient{
		baseURL: config.ServerURL,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
	}
}

// List 通过服务端 API 列表查询
func (c *nodeExecClient) List(ctx context.Context, opts ListOptions) (*NodeExecList, error) {
	url := fmt.Sprintf("%s/pipe_exec/list?namespace=%s&kind=%s", c.baseURL, opts.Namespace, opts.Kind)
	if opts.Limit > 0 {
		url += fmt.Sprintf("&limit=%d", opts.Limit)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call server API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned status %d", resp.StatusCode)
	}

	// 解析服务端的响应格式
	var response struct {
		Code   int           `json:"code"`
		ErrMsg string        `json:"err_msg"`
		Info   *NodeExecList `json:"info"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if response.Code != 0 {
		return nil, fmt.Errorf("server error: %s", response.ErrMsg)
	}

	if response.Info == nil {
		return &NodeExecList{Items: []*model.NodeExec{}}, nil
	}

	return response.Info, nil
}

// Get 通过服务端 API 获取单个资源
func (c *nodeExecClient) Get(ctx context.Context, namespace, kind string, id int64) (*model.NodeExec, error) {
	url := fmt.Sprintf("%s/pipe_exec/get?namespace=%s&kind=%s&id=%d", c.baseURL, namespace, kind, id)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call server API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned status %d", resp.StatusCode)
	}

	// 解析服务端的响应格式
	var response struct {
		Code   int             `json:"code"`
		ErrMsg string          `json:"err_msg"`
		Info   *model.NodeExec `json:"info"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if response.Code != 0 {
		return nil, fmt.Errorf("server error: %s", response.ErrMsg)
	}

	return response.Info, nil
}

// Watch 通过服务端 API 监听资源变化（包含 list and watch）
func (c *nodeExecClient) Watch(ctx context.Context, opts WatchOptions) (WatchInterface, error) {
	w := &watcher{
		namespace:       opts.Namespace,
		kind:            opts.Kind,
		resourceVersion: opts.ResourceVersion,
		resultChan:      make(chan Event, 100),
		stopChan:        make(chan struct{}),
		client:          c,
	}

	watcherID := fmt.Sprintf("%s-%s-%d", opts.Namespace, opts.Kind, time.Now().UnixNano())
	c.watchers.Store(watcherID, w)

	go w.start(ctx, watcherID)

	return w, nil
}

// watcher 监听器实现
type watcher struct {
	namespace       string
	kind            string
	resourceVersion string
	resultChan      chan Event
	stopChan        chan struct{}
	client          *nodeExecClient
	stopped         bool
	mu              sync.RWMutex
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

	// 通过服务端 API 监听
	w.watchServer(ctx)
}

// watchServer 通过服务端 API 监听变化
func (w *watcher) watchServer(ctx context.Context) {
	url := fmt.Sprintf("%s/operator/namespace/%s/kind/%s", w.client.baseURL, w.namespace, w.kind)
	
	// 添加 resourceVersion 参数
	if w.resourceVersion != "" {
		url += fmt.Sprintf("?resourceVersion=%s", w.resourceVersion)
	}

	for {
		select {
		case <-w.stopChan:
			return
		case <-ctx.Done():
			return
		default:
			if err := w.connectAndWatch(ctx, url); err != nil {
				log.Errorf("watch connection failed: %v", err)
				w.sendEvent(Event{
					Type:  EventTypeError,
					Error: err,
				})
				time.Sleep(5 * time.Second) // 重连延迟
			}
		}
	}
}

// connectAndWatch 连接服务端并监听事件
func (w *watcher) connectAndWatch(ctx context.Context, url string) error {
	log.Infof("🔗 Connecting to watch server: %s", url)
	
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := w.client.httpClient.Do(req)
	if err != nil {
		log.Errorf("❌ Failed to connect to server: %v", err)
		return fmt.Errorf("failed to connect to server: %w", err)
	}
	defer resp.Body.Close()

	log.Infof("📡 HTTP response status: %d", resp.StatusCode)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned status %d", resp.StatusCode)
	}

	log.Infof("✅ Watch connection established, reading events...")
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		select {
		case <-w.stopChan:
			log.Infof("🛑 Watch stopped by client")
			return nil
		case <-ctx.Done():
			log.Infof("🛑 Watch stopped by context")
			return ctx.Err()
		default:
			line := scanner.Text()
			if line == "" {
				continue
			}

			log.Debugf("📨 Received raw event: %s", line)
			var event Event
			if err := json.Unmarshal([]byte(line), &event); err != nil {
				log.Warnf("❌ Failed to unmarshal event: %v, raw: %s", err, line)
				continue
			}

			log.Infof("📨 Parsed event: type=%s", event.Type)
			w.sendEvent(event)
		}
	}

	if err := scanner.Err(); err != nil {
		log.Errorf("❌ Scanner error: %v", err)
		return fmt.Errorf("scanner error: %w", err)
	}

	log.Infof("🔌 Watch connection closed normally")
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