package model

// GetNamespace 实现Object接口
func (n *NodeExec) GetNamespace() string {
	return n.Namespace
}

// GetKind 实现Object接口
func (n *NodeExec) GetKind() string {
	return n.Kind
}

// GetID 实现Object接口
func (n *NodeExec) GetID() int64 {
	return n.Id
}