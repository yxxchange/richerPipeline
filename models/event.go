package models

type EventType string

const (
	EventTypeNodeCreate EventType = "node_create"
	EventTypeNodeDelete EventType = "node_delete"
	EventTypeNodePatch  EventType = "node_patch"
	EventTypeNodePut    EventType = "node_put"
)

type Event struct {
	EventType
	Object
}
