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
	Replace(items map[string]*model.NodeExec)
}

// store 本地存储实现
type store struct {
	mu    sync.RWMutex
	items map[string]*model.NodeExec
	// 索引用于快速查找
	namespaceIndex map[string]map[string]*model.NodeExec // namespace -> key -> obj
	kindIndex      map[string]map[string]*model.NodeExec // kind -> key -> obj
}

// NewStore 创建存储
func NewStore() Store {
	return &store{
		items:          make(map[string]*model.NodeExec),
		namespaceIndex: make(map[string]map[string]*model.NodeExec),
		kindIndex:      make(map[string]map[string]*model.NodeExec),
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
	
	// 优化查找策略
	if namespace != "" && kind != "" {
		// 两个条件都有，使用交集
		namespaceObjs := s.namespaceIndex[namespace]
		kindObjs := s.kindIndex[kind]
		
		for key, obj := range namespaceObjs {
			if _, exists := kindObjs[key]; exists {
				result = append(result, s.copyObject(obj))
			}
		}
	} else if namespace != "" {
		// 只有 namespace 条件
		for _, obj := range s.namespaceIndex[namespace] {
			result = append(result, s.copyObject(obj))
		}
	} else if kind != "" {
		// 只有 kind 条件
		for _, obj := range s.kindIndex[kind] {
			result = append(result, s.copyObject(obj))
		}
	} else {
		// 没有条件，返回所有
		for _, obj := range s.items {
			result = append(result, s.copyObject(obj))
		}
	}
	
	return result
}

// Add 添加对象
func (s *store) Add(key string, obj *model.NodeExec) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	copiedObj := s.copyObject(obj)
	s.items[key] = copiedObj
	
	// 更新索引
	s.addToIndex(key, copiedObj)
}

// Update 更新对象
func (s *store) Update(key string, obj *model.NodeExec) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	// 先从索引中移除旧对象
	if oldObj, exists := s.items[key]; exists {
		s.removeFromIndex(key, oldObj)
	}
	
	// 添加新对象
	copiedObj := s.copyObject(obj)
	s.items[key] = copiedObj
	s.addToIndex(key, copiedObj)
}

// Delete 删除对象
func (s *store) Delete(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	// 从索引中移除
	if obj, exists := s.items[key]; exists {
		s.removeFromIndex(key, obj)
	}
	
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

// Replace 替换所有对象
func (s *store) Replace(items map[string]*model.NodeExec) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	// 清空现有数据
	s.items = make(map[string]*model.NodeExec)
	s.namespaceIndex = make(map[string]map[string]*model.NodeExec)
	s.kindIndex = make(map[string]map[string]*model.NodeExec)
	
	// 添加新数据
	for key, obj := range items {
		copiedObj := s.copyObject(obj)
		s.items[key] = copiedObj
		s.addToIndex(key, copiedObj)
	}
}

// addToIndex 添加到索引
func (s *store) addToIndex(key string, obj *model.NodeExec) {
	// 添加到 namespace 索引
	if obj.Namespace != "" {
		if s.namespaceIndex[obj.Namespace] == nil {
			s.namespaceIndex[obj.Namespace] = make(map[string]*model.NodeExec)
		}
		s.namespaceIndex[obj.Namespace][key] = obj
	}
	
	// 添加到 kind 索引
	if obj.Kind != "" {
		if s.kindIndex[obj.Kind] == nil {
			s.kindIndex[obj.Kind] = make(map[string]*model.NodeExec)
		}
		s.kindIndex[obj.Kind][key] = obj
	}
}

// removeFromIndex 从索引中移除
func (s *store) removeFromIndex(key string, obj *model.NodeExec) {
	// 从 namespace 索引中移除
	if obj.Namespace != "" {
		if nsIndex := s.namespaceIndex[obj.Namespace]; nsIndex != nil {
			delete(nsIndex, key)
			if len(nsIndex) == 0 {
				delete(s.namespaceIndex, obj.Namespace)
			}
		}
	}
	
	// 从 kind 索引中移除
	if obj.Kind != "" {
		if kindIndex := s.kindIndex[obj.Kind]; kindIndex != nil {
			delete(kindIndex, key)
			if len(kindIndex) == 0 {
				delete(s.kindIndex, obj.Kind)
			}
		}
	}
}