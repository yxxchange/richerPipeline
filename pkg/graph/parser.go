package graph

import (
	"fmt"
	"github.com/yxxchange/richerLog/log"
	"github.com/yxxchange/richerPipeline/models"
	"github.com/yxxchange/richerPipeline/pkg/common"
	"github.com/yxxchange/richerPipeline/pkg/sort"
)

type IParser interface {
	Parse(raw RawPipeline) (graph WorkGraph, err error)
	Validate(raw RawPipeline) error
}

func NewParser(version string, pipeType string) (IParser, error) {
	switch version {
	case common.VersionV1:
		return newV1Parser(pipeType)
	default:
		return nil, common.WrapError(common.ErrPipelineVersion, fmt.Errorf("unsupported version: %s", version))
	}
}

func newV1Parser(pipeType string) (IParser, error) {
	switch pipeType {
	case models.DefaultPipeline:
		return &GeneralParser{}, nil
	default:
		return nil, common.WrapError(common.ErrPipelineType, fmt.Errorf("unsupported pipeline type: %s", pipeType))
	}
}

type GeneralParser struct {
}

func (p *GeneralParser) Validate(raw RawPipeline) error {
	nodeMap := extractNode(raw)
	edgeMap := extractEdge(raw)
	_, err := GenDAGraph(nodeMap, edgeMap)
	if err != nil {
		log.Errorf("生成DAG图失败: %v", err)
		return common.WrapError(common.ErrDataNotDAG, err)
	}
	return nil
}

func (p *GeneralParser) Parse(raw RawPipeline) (WorkGraph, error) {
	nodeMap := extractNode(raw)
	edgeMap := extractEdge(raw)
	graph, err := GenDAGraph(nodeMap, edgeMap)
	if err != nil {
		log.Errorf("生成DAG图失败: %v", err)
		return WorkGraph{}, err
	}
	return WorkGraph{
		Metadata: raw.Metadata,
		DAGraph:  graph,
		RawData:  raw,
	}, nil
}

func extractNode(pipeline RawPipeline) map[string]*NodeInfo {
	nodeMap := make(map[string]*NodeInfo)
	for _, node := range pipeline.Graph.Nodes {
		nodeMap[node.Name] = &node
	}
	return nodeMap
}

func extractEdge(pipeline RawPipeline) map[string][]string {
	edgeMap := make(map[string][]string)
	for _, edge := range pipeline.Graph.Edges {
		edgeMap[edge.Source] = append(edgeMap[edge.Source], edge.Target)
	}
	return edgeMap
}

func GenDAGraph(nodeMap map[string]*NodeInfo, edgeMap map[string][]string) (res WorkDAGraph, err error) {
	defer func() {
		if r := recover(); r != nil {
			log.Infof("GenDAGraph panic: %v", err)
			err = fmt.Errorf("GenDAGraph panic: %v", err)
		}
	}()
	err = validate(nodeMap, edgeMap)
	if err != nil {
		return WorkDAGraph{}, err
	}
	resMap := make(map[string]*WorkNode)
	for _, node := range nodeMap {
		newNode := &WorkNode{
			WorkerEngine: node.Ctx.Input.Worker,
			Self:         node,
			Child:        make([]*NodeInfo, 0),
		}
		if _, ok := resMap[node.Name]; ok {
			newNode = resMap[node.Name]
		}
		for _, target := range edgeMap[node.Name] {
			if _, ok := resMap[target]; !ok {
				resMap[target] = &WorkNode{
					WorkerEngine: node.Ctx.Input.Worker,
					Self:         nodeMap[target],
					Child:        make([]*NodeInfo, 0),
				}
			}
			newNode.Child = append(newNode.Child, nodeMap[target])
		}
		resMap[node.Name] = newNode
	}
	tmp := &WorkDAGraph{
		Map: resMap,
	}
	tmp = tmp.GenExtendInfo()
	sorted, err := sort.TopologicalSort(tmp)
	if err != nil {
		return WorkDAGraph{}, common.WrapError(common.ErrDataNotDAG, err)
	}
	tmp = sorted.(*WorkDAGraph)
	res = *tmp
	return
}

func validate(nodeMap map[string]*NodeInfo, edgeMap map[string][]string) error {
	if len(nodeMap) == 0 {
		return fmt.Errorf("empty node map")
	}
	for source, targets := range edgeMap {
		if _, ok := nodeMap[source]; !ok {
			return fmt.Errorf("source node %s not found", source)
		}
		for _, target := range targets {
			if _, ok := nodeMap[target]; !ok {
				return fmt.Errorf("target node %s not found", target)
			}
		}
	}
	return nil
}
