package nodeexec

import (
	"context"

	"github.com/yxxchange/pipefree/client/cache"
	"github.com/yxxchange/pipefree/client/listers/nodeexec/v1"
	"github.com/yxxchange/pipefree/infra/dal/model"
)

// Interface 节点执行Informer接口
type Interface interface {
	V1() V1Interface
}

// V1Interface v1版本接口
type V1Interface interface {
	NodeExecs() NodeExecInformer
}

// SharedInformer 共享Informer接口
type SharedInformer interface {
	AddEventHandler(handler ResourceEventHandler)
	Run(ctx context.Context) error
	HasSynced() bool
	GetStore() cache.Store
}

// ResourceEventHandler 资源事件处理器
type ResourceEventHandler interface {
	OnAdd(obj *model.NodeExec)
	OnUpdate(oldObj, newObj *model.NodeExec)
	OnDelete(obj *model.NodeExec)
}

// ResourceEventHandlerFuncs 资源事件处理函数
type ResourceEventHandlerFuncs struct {
	AddFunc    func(obj *model.NodeExec)
	UpdateFunc func(oldObj, newObj *model.NodeExec)
	DeleteFunc func(obj *model.NodeExec)
}

// OnAdd 添加事件
func (r ResourceEventHandlerFuncs) OnAdd(obj *model.NodeExec) {
	if r.AddFunc != nil {
		r.AddFunc(obj)
	}
}

// OnUpdate 更新事件
func (r ResourceEventHandlerFuncs) OnUpdate(oldObj, newObj *model.NodeExec) {
	if r.UpdateFunc != nil {
		r.UpdateFunc(oldObj, newObj)
	}
}

// OnDelete 删除事件
func (r ResourceEventHandlerFuncs) OnDelete(obj *model.NodeExec) {
	if r.DeleteFunc != nil {
		r.DeleteFunc(obj)
	}
}

// NodeExecInformer 节点执行Informer接口
type NodeExecInformer interface {
	Informer() SharedInformer
	Lister() v1.NodeExecLister
}

// group 组实现
type group struct {
	factory informers.SharedInformerFactory
}

// New 创建新的组
func New(f informers.SharedInformerFactory) Interface {
	return &group{factory: f}
}

// V1 获取v1版本接口
func (g *group) V1() V1Interface {
	return &version{factory: g.factory}
}

// version v1版本实现
type version struct {
	factory informers.SharedInformerFactory
}

// NodeExecs 获取节点执行Informer
func (v *version) NodeExecs() NodeExecInformer {
	return &nodeExecInformer{factory: v.factory}
}

// nodeExecInformer 节点执行Informer实现
type nodeExecInformer struct {
	factory interface {
		GetInformer(namespace, kind string) interface {
			AddEventHandler(handler interface{})
			Run(ctx context.Context) error
			HasSynced() bool
			GetStore() cache.Store
		}
	}
}

// Informer 获取Informer
func (n *nodeExecInformer) Informer() SharedInformer {
	return &sharedInformerWrapper{n.factory.GetInformer("", "")}
}

// Lister 获取列表器
func (n *nodeExecInformer) Lister() v1.NodeExecLister {
	return v1.NewNodeExecLister(n.Informer().GetStore())
}

// sharedInformerWrapper 包装器
type sharedInformerWrapper struct {
	informer interface {
		AddEventHandler(handler interface{})
		Run(ctx context.Context) error
		HasSynced() bool
		GetStore() cache.Store
	}
}

func (w *sharedInformerWrapper) AddEventHandler(handler ResourceEventHandler) {
	w.informer.AddEventHandler(handler)
}

func (w *sharedInformerWrapper) Run(ctx context.Context) error {
	return w.informer.Run(ctx)
}

func (w *sharedInformerWrapper) HasSynced() bool {
	return w.informer.HasSynced()
}

func (w *sharedInformerWrapper) GetStore() cache.Store {
	return w.informer.GetStore()
}