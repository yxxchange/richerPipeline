package graph

import (
	"github.com/yxxchange/richerPipeline/models"
	"github.com/yxxchange/richerPipeline/pkg"
)

// WorkNode 用户侧的PipelineCfg，每一个工作节点对应一个工作节点
type WorkNode struct {
	WorkerEngine string
	Self         *models.NodeInfo
	Child        []*models.NodeInfo
	extendInfo   ExtendNodeInfo
}

// WorkGraph 图结构，包含了元数据、DAG图、原始数据
type WorkGraph struct {
	Metadata models.Metadata
	DAGraph  WorkDAGraph
	RawData  models.RawPipeline
}

// WorkDAGraph 用于拓扑排序的图结构,同时保留了节点的业务信息
type WorkDAGraph struct {
	Map              map[string]*WorkNode
	topologicalSlice []*WorkNode // 用于存储拓扑排序的结果
}

type ExtendNodeInfo struct {
	InDegree int
}

var _ pkg.ITopologicalSorter = &WorkDAGraph{}

func (w *WorkDAGraph) GenExtendInfo() *WorkDAGraph {
	for _, node := range w.Map {
		node.extendInfo.InDegree = 0
	}
	for _, node := range w.Map {
		for _, child := range node.Child {
			w.Map[child.Name].extendInfo.InDegree++
		}
	}
	return w
}

func (w *WorkDAGraph) Count() int {
	return len(w.Map)
}

func (w *WorkDAGraph) InDegreeIsZero(index interface{}) bool {
	node := w.Map[index.(string)]
	return node.extendInfo.InDegree == 0
}

func (w *WorkDAGraph) InDegreeSubOne(index interface{}) error {
	node := w.Map[index.(string)]
	node.extendInfo.InDegree--
	return nil
}

func (w *WorkDAGraph) Children(index interface{}) []interface{} {
	node := w.Map[index.(string)]
	res := make([]interface{}, 0)
	for _, child := range node.Child {
		res = append(res, child.Name)
	}
	return res
}

func (w *WorkDAGraph) AddElement(index interface{}) {
	w.topologicalSlice = append(w.topologicalSlice, w.Map[index.(string)])
}

func (w *WorkDAGraph) Index() []interface{} {
	res := make([]interface{}, 0)
	for key := range w.Map {
		res = append(res, key)
	}
	return res
}

func (w *WorkDAGraph) OutputTopologicalSlice() []string {
	res := make([]string, 0)
	for _, node := range w.topologicalSlice {
		res = append(res, node.Self.Name)
	}
	return res
}
