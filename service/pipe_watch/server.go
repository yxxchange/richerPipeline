package pipe_watch

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"

	"github.com/yxxchange/pipefree/helper/log"
	"github.com/yxxchange/pipefree/helper/safe"
	"github.com/yxxchange/pipefree/infra/etcd"
	clientv3 "go.etcd.io/etcd/client/v3"
)

var serverInstance *WatchServer
var once sync.Once

func GetWatchServer() *WatchServer {
	once.Do(func() {
		serverInstance = NewWatchServer()
	})
	return serverInstance
}

type WatchServer struct {
	lock      sync.Mutex
	ctx       context.Context
	streamAgg map[string]*EventStream
}

func NewWatchServer() *WatchServer {
	return &WatchServer{
		ctx:       context.Background(),
		streamAgg: make(map[string]*EventStream),
	}
}

type EventStream struct {
	lock   sync.RWMutex
	ctx    context.Context
	cancel context.CancelFunc

	watchStarted bool // 是否已启动监听
	revSince     int64
	streamId     string
	channels     map[string]*EventChannel
}

func NewEventStream(ctx context.Context, prefix string) *EventStream {
	son, cancel := context.WithCancel(ctx)
	return &EventStream{
		cancel:   cancel,
		ctx:      son,
		streamId: prefix,
	}
}

func (s *WatchServer) ListAndWatch(operator OperatorCtx) {
	stream := s.GetEventStream(operator.StreamID)
	err := stream.ListAndWatch(operator.UUID, operator.EventChannel)
	if err != nil { // 处理错误
		errMsg := fmt.Errorf("failed to list and watch streamId %s: %v", stream.streamId, err)
		operator.EventChannel.SendErr(errMsg)
		operator.EventChannel.Close()
		return
	}
}

func (s *WatchServer) GetEventStream(prefix string) *EventStream {
	s.lock.Lock()
	defer s.lock.Unlock()
	if s.streamAgg == nil {
		s.streamAgg = make(map[string]*EventStream)
	}
	if stream, exists := s.streamAgg[prefix]; exists {
		return stream
	}
	stream := NewEventStream(s.ctx, prefix)
	s.streamAgg[prefix] = stream
	return stream
}

func (s *WatchServer) RemoveOperator(operator OperatorCtx) {
	s.lock.Lock()
	defer s.lock.Unlock()
	if stream, exists := s.streamAgg[operator.StreamID]; exists {
		stream.RemoveChannel(operator.UUID)
		if len(stream.channels) == 0 {
			log.Infof("No more channels for streamId %s, stopping watch", operator.StreamID)
			stream.cancel()
			delete(s.streamAgg, operator.StreamID) // 删除无效的事件流
		}
	}
}

func (h *EventStream) RemoveChannel(uuid string) {
	h.lock.Lock()
	defer h.lock.Unlock()

	if h.channels != nil {
		if _, exists := h.channels[uuid]; exists {
			delete(h.channels, uuid) // 删除通道
		}
	}
}

func (h *EventStream) ListAndWatch(uuid string, ch *EventChannel) error {
	return h.ListAndWatchWithRevision(uuid, ch, "")
}

func (h *EventStream) ListAndWatchWithRevision(uuid string, ch *EventChannel, resourceVersion string) error {
	// 如果指定了 resourceVersion，直接从该版本开始 watch，跳过 list
	if resourceVersion != "" {
		if rev, err := strconv.ParseInt(resourceVersion, 10, 64); err == nil {
			h.revSince = rev
			log.Infof("Starting watch from resourceVersion %s for streamId %s", resourceVersion, h.streamId)
		} else {
			log.Warnf("Invalid resourceVersion %s, starting from current revision", resourceVersion)
		}
		h.AddChannel(uuid, ch)
		if !h.watchStarted {
			h.watchStarted = true
			safe.Go(h.Watch)
		}
		return nil
	}

	// 没有指定 resourceVersion，执行完整的 list and watch
	rev, err := h.List(ch)
	if err != nil {
		return fmt.Errorf("failed to list streamId %s: %v", h.streamId, err)
	}
	h.AddChannel(uuid, ch)
	if !h.watchStarted {
		h.watchStarted = true // 标记监听已启动
		h.revSince = rev
		safe.Go(h.Watch)
	}
	return nil
}

func (h *EventStream) AddChannel(uuid string, ch *EventChannel) {
	h.lock.Lock()
	defer h.lock.Unlock()
	if h.channels == nil {
		h.channels = make(map[string]*EventChannel) // 初始化通道切片
	}
	h.channels[uuid] = ch
}

func (h *EventStream) Remove(uuid string) {
	h.lock.Lock()
	defer h.lock.Unlock()
	delete(h.channels, uuid)
}

func (h *EventStream) List(ch *EventChannel) (int64, error) {
	resp, err := etcd.GetWithPrefix(h.ctx, h.streamId)
	if err != nil {
		log.Errorf("failed to get streamId %s: %v", h.streamId, err)
		return 0, err
	}
	h.HandleList(resp, ch)
	return resp.Header.Revision, nil
}

func (h *EventStream) Watch() {
	etcd.Watch(h.ctx, h.streamId, h.revSince, h.HandleWatch)
}

func (h *EventStream) HandleList(result *clientv3.GetResponse, ch *EventChannel) {
	log.Infof("Handling list response with %d items for streamId %s", len(result.Kvs), h.streamId)

	for _, kv := range result.Kvs {
		// 转换为客户端期望的事件格式
		clientEvent := h.convertToClientEvent(&clientv3.Event{
			Type: clientv3.EventTypePut,
			Kv:   kv,
		})

		b, err := json.Marshal(clientEvent)
		if err != nil {
			log.Errorf("failed to marshal event: %v", err)
			continue
		}

		// 添加换行符
		b = append(b, '\n')

		log.Debugf("Sending list event: key=%s", string(kv.Key))

		select {
		case ch.ch <- b:
		default:
			log.Warnf("channel buffer full, dropping list event for streamId %s", h.streamId)
		}
	}
}

// convertToClientEvent 转换为客户端期望的事件格式
func (h *EventStream) convertToClientEvent(etcdEvent *clientv3.Event) map[string]interface{} {
	if etcdEvent == nil || etcdEvent.Kv == nil {
		return map[string]interface{}{
			"type":   "ERROR",
			"object": nil,
			"error":  "received nil event or nil KeyValue",
		}
	}

	// 解析 NodeExec 对象
	var nodeExec map[string]interface{}
	if etcdEvent.Kv.Value != nil {
		if err := json.Unmarshal(etcdEvent.Kv.Value, &nodeExec); err != nil {
			log.Errorf("failed to unmarshal node exec: %v", err)
			return map[string]interface{}{
				"type":   "ERROR",
				"object": nil,
				"error":  fmt.Sprintf("failed to unmarshal: %v", err),
			}
		}
	}

	// 确定事件类型
	var eventType string
	switch etcdEvent.Type {
	case clientv3.EventTypePut:
		if etcdEvent.Kv.CreateRevision == etcdEvent.Kv.ModRevision {
			eventType = "ADDED"
		} else {
			eventType = "MODIFIED"
		}
	case clientv3.EventTypeDelete:
		eventType = "DELETED"
	default:
		eventType = "UNKNOWN"
	}

	return map[string]interface{}{
		"type":   eventType,
		"object": nodeExec,
	}
}

func (h *EventStream) HandleWatch(result *clientv3.WatchResponse, closed bool) {
	if closed {
		log.Infof("watch closed for streamId %s", h.streamId)
		for _, ch := range h.channels {
			ch.done <- struct{}{}
			ch.Close() // 关闭通道
		}
		return
	}

	if result.Err() != nil {
		log.Errorf("watch error for streamId %s: %v", h.streamId, result.Err())
		return
	}

	log.Debugf("Received %d watch events for streamId %s", len(result.Events), h.streamId)

	for _, e := range result.Events {
		// 转换为客户端期望的事件格式
		clientEvent := h.convertToClientEvent(e)
		b, err := json.Marshal(clientEvent)
		if err != nil {
			log.Errorf("failed to marshal event: %v", err)
			continue
		}

		// 添加换行符，确保客户端能正确解析
		b = append(b, '\n')

		log.Debugf("Sending event: type=%s, key=%s", clientEvent, string(e.Kv.Key))

		for _, ch := range h.channels {
			select {
			case ch.ch <- b:
			default:
				log.Warnf("channel buffer full, dropping event for streamId %s", h.streamId)
			}
		}
	}
}
