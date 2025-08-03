package v1

import (
	"fmt"

	"github.com/yxxchange/pipefree/client/cache"
	"github.com/yxxchange/pipefree/infra/dal/model"
)

// NodeExecLister 节点执行列表器接口
type NodeExecLister interface {
	List(namespace, kind string) ([]*model.NodeExec, error)
	Get(namespace, kind string, id int64) (*model.NodeExec, error)
}

// nodeExecLister 节点执行列表器实现
type nodeExecLister struct {
	store cache.Store
}

// NewNodeExecLister 创建节点执行列表器
func NewNodeExecLister(store cache.Store) NodeExecLister {
	return &nodeExecLister{
		store: store,
	}
}

// List 列出对象
func (n *nodeExecLister) List(namespace, kind string) ([]*model.NodeExec, error) {
	return n.store.GetByNamespaceAndKind(namespace, kind), nil
}

// Get 获取对象
func (n *nodeExecLister) Get(namespace, kind string, id int64) (*model.NodeExec, error) {
	key := getObjectKey(namespace, kind, id)
	obj, exists := n.store.Get(key)
	if !exists {
		return nil, fmt.Errorf("node exec not found: %s", key)
	}
	return obj, nil
}

// getObjectKey 获取对象键
func getObjectKey(namespace, kind string, id int64) string {
	return fmt.Sprintf("%s/%s/%d", namespace, kind, id)
}