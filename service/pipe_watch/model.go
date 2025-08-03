package pipe_watch

import (
	"encoding/json"
	"fmt"
	"github.com/yxxchange/pipefree/helper/log"
	"github.com/yxxchange/pipefree/infra/dal/model"
	clientv3 "go.etcd.io/etcd/client/v3"
	"sync"
)

type EventChannel struct {
	once   sync.Once
	ch     chan []byte
	done   chan struct{}
	errMsg chan error
}

func NewEventChannel() *EventChannel {
	return &EventChannel{
		ch:     make(chan []byte, 100),
		done:   make(chan struct{}),
		errMsg: make(chan error, 100),
	}
}

func (ec *EventChannel) Ch() <-chan []byte {
	return ec.ch
}

func (ec *EventChannel) ErrCh() <-chan error {
	return ec.errMsg
}

func (ec *EventChannel) Done() <-chan struct{} {
	return ec.done
}

func (ec *EventChannel) Close() {
	ec.once.Do(func() {
		close(ec.done)
		close(ec.ch)
		close(ec.errMsg)
	})
}

func (ec *EventChannel) SendErr(err error) {
	ec.errMsg <- err
}

type EventType string

const (
	EventTypeUpdate EventType = "update" // 更新事件
	EventTypeDelete EventType = "delete" // 删除事件
	EventTypeCreate EventType = "create" // 创建事件
)

type Event struct {
	Type         EventType      `json:"type"`          // 事件类型
	NodeExec     model.NodeExec `json:"node_exec"`     // 节点执行信息
	ErrMsg       string         `json:"err_msg"`       // 错误消息
	Revision     int64          `json:"revision"`      // 事件的版本号
	StreamClosed bool           `json:"stream_closed"` // 是否为流关闭事件
}

func CloseEvent() Event {
	return Event{
		Type:         EventTypeUpdate,
		NodeExec:     model.NodeExec{},
		ErrMsg:       "",
		Revision:     0,
		StreamClosed: true, // 标记为流关闭事件
	}
}

func (e *Event) IsStreamClosed() bool {
	return e.StreamClosed
}

func (e *Event) Err() error {
	if e.ErrMsg != "" {
		return fmt.Errorf(e.ErrMsg)
	}
	return nil
}

func Convert(event *clientv3.Event) Event {
	if event == nil || event.Kv == nil {
		log.Errorf("received nil event or nil KeyValue")
		return Event{
			ErrMsg: "received nil event or nil KeyValue",
		} // 返回空事件或处理错误
	}
	var nodeExec model.NodeExec
	if event.Kv.Value != nil {
		if err := json.Unmarshal(event.Kv.Value, &nodeExec); err != nil {
			log.Errorf("failed to unmarshal event value: %v", err)
			return Event{
				ErrMsg: "failed to unmarshal event value: " + err.Error(),
			} // 返回空事件或处理错误
		}
	}

	var eventType EventType
	switch event.Type {
	case clientv3.EventTypePut:
		if event.Kv.CreateRevision == event.Kv.ModRevision {
			eventType = EventTypeCreate
		} else {
			eventType = EventTypeUpdate
		}
	case clientv3.EventTypeDelete:
		eventType = EventTypeDelete
	default:
		return Event{
			ErrMsg: fmt.Sprintf("unknown event type: %s", event.Type),
		}
	}

	return Event{
		Type:     eventType,
		NodeExec: nodeExec,
		ErrMsg:   "",
		Revision: event.Kv.Version,
	}
}

type OperatorCtx struct {
	StreamID     string
	UUID         string
	EventChannel *EventChannel
}

func NewOperatorCtx(streamID, uuid string) OperatorCtx {
	return OperatorCtx{
		StreamID:     streamID,
		UUID:         uuid,
		EventChannel: NewEventChannel(),
	}
}
