package cache

import (
	"sync"

	"github.com/yxxchange/pipefree/infra/dal/model"
)

// Store 本地存储接口
type Store interface {
	Get(key string) (*model.NodeExec, bool)
	List() []*model.NodeExec
	GetByNamespaceAndKind(namespace, kind string) []*model.NodeExec
	Add(key string, obj *model.NodeExec)
	Update(key string, obj *model.NodeExec)
	Delete(key string)
	ListKeys() []string
}

// store 本地存储实现
type store struct {
	mu    sync.RWMutex
	items map[string]*model.NodeExec
}

// NewStore 创建存储
func NewStore() Store {
	return &store{
		items: make(map[string]*model.NodeExec),
	}
}

// Get 获取对象
func (s *store) Get(key string) (*model.NodeExec, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	obj, exists := s.items[key]
	if !exists {
		return nil, false
	}
	
	return s.copyObject(obj), true
}

// List 列出所有对象
func (s *store) List() []*model.NodeExec {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	result := make([]*model.NodeExec, 0, len(s.items))
	for _, obj := range s.items {
		result = append(result, s.copyObject(obj))
	}
	
	return result
}

// GetByNamespaceAndKind 根据命名空间和类型获取对象
func (s *store) GetByNamespaceAndKind(namespace, kind string) []*model.NodeExec {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	var result []*model.NodeExec
	for _, obj := range s.items {
		if (namespace == "" || obj.Namespace == namespace) &&
		   (kind == "" || obj.Kind == kind) {
			result = append(result, s.copyObject(obj))
		}
	}
	
	return result
}

// Add 添加对象
func (s *store) Add(key string, obj *model.NodeExec) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	s.items[key] = s.copyObject(obj)
}

// Update 更新对象
func (s *store) Update(key string, obj *model.NodeExec) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	s.items[key] = s.copyObject(obj)
}

// Delete 删除对象
func (s *store) Delete(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	delete(s.items, key)
}

// ListKeys 列出所有键
func (s *store) ListKeys() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	keys := make([]string, 0, len(s.items))
	for key := range s.items {
		keys = append(keys, key)
	}
	
	return keys
}

// copyObject 复制对象
func (s *store) copyObject(obj *model.NodeExec) *model.NodeExec {
	if obj == nil {
		return nil
	}
	
	copied := &model.NodeExec{
		Basic:       obj.Basic,
		Name:        obj.Name,
		Kind:        obj.Kind,
		Namespace:   obj.Namespace,
		Version:     obj.Version,
		PipeSpace:   obj.PipeSpace,
		PipeName:    obj.PipeName,
		PipeVersion: obj.PipeVersion,
		NodeCfgId:   obj.NodeCfgId,
		PipeCfgId:   obj.PipeCfgId,
		PipeExecId:  obj.PipeExecId,
		InDegree:    obj.InDegree,
		Spec:        obj.Spec,
		Phase:       obj.Phase,
	}
	
	return copied
}