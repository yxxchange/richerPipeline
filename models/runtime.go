package models

import (
	"encoding/json"
	"github.com/yxxchange/richerPipeline/pkg/sort"
)

// WorkNode 用户侧的PipelineCfg，每一个工作节点对应一个工作节点
type WorkNode struct {
	WorkerEngine string
	Self         *NodeInfo
	Child        []*NodeInfo
	extendInfo   ExtendNodeInfo
}

// WorkGraph 图结构，包含了元数据、DAG图、原始数据
type WorkGraph struct {
	Metadata Metadata
	DAGraph  WorkDAGraph
	RawData  RawPipeline
}

// WorkDAGraph 用于拓扑排序的图结构,同时保留了节点的业务信息
type WorkDAGraph struct {
	Map              map[string]*WorkNode
	topologicalSlice []*WorkNode // 用于存储拓扑排序的结果
}

type ExtendNodeInfo struct {
	InDegree int
}

type RawPipeline struct {
	PipelineVersion string   `yaml:"pipelineVersion"`
	Metadata        Metadata `yaml:"metadata"`
	Graph           RawGraph `yaml:"graph"`
}

type Metadata struct {
	Name      string `yaml:"name"`
	Namespace string `yaml:"namespace"`
}

type RawGraph struct {
	Nodes []NodeInfo `yaml:"nodes"`
	Edges []EdgeInfo `yaml:"edges"`
}

type NodeInfo struct {
	Name   string  `yaml:"name"`
	Ctx    Context `yaml:"ctx"`
	Config Config  `yaml:"config"`
	Status Status  `yaml:"status"`
}

type Context struct {
	Input Input `yaml:"input"`
}

var _ Object = &Context{}

func (c Context) DeepCopyObject() Object {
	res, err := c.DeepCopy()
	if err != nil {
		return nil
	}
	return res
}

func (c Context) DeepCopy() (Object, error) {
	b, err := json.Marshal(c)
	if err != nil {
		return nil, err
	}
	var res Context
	err = json.Unmarshal(b, &res)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (c Context) ObjectType() ObjectType {
	return ObjectTypeNode
}

type Input struct {
	Worker    string `yaml:"worker"`
	JsonParam string `yaml:"jsonParam"`
}

type Config struct {
	Retry           int    `yaml:"retry"`
	Timeout         int    `yaml:"timeout"`
	TimeoutPolicy   string `yaml:"timeoutPolicy"`
	SchedulerPolicy string `yaml:"schedulerPolicy"`
}

type Status struct {
	State       string `yaml:"state"`
	StartTime   uint64 `yaml:"startTime"`
	EndTime     uint64 `yaml:"endTime"`
	Duration    uint64 `yaml:"duration"`
	ErrMsg      string `yaml:"errMsg"`
	Data        string `yaml:"data"`
	ExecutorUid string `yaml:"-"`
}

type EdgeInfo struct {
	Source string `yaml:"source"`
	Target string `yaml:"target"`
}

var _ sort.ITopologicalSorter = &WorkDAGraph{}

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
