package client

import (
	"github.com/yxxchange/pipefree/client/typed"
)

// Interface 客户端接口
type Interface interface {
	NodeExecs() typed.NodeExecInterface
}

// Clientset 客户端集合
type Clientset struct {
	nodeExecs typed.NodeExecInterface
}

// NewForConfig 创建客户端
func NewForConfig() Interface {
	return &Clientset{
		nodeExecs: typed.NewNodeExecClient(),
	}
}

// NodeExecs 获取节点执行接口
func (c *Clientset) NodeExecs() typed.NodeExecInterface {
	return c.nodeExecs
}