package graph

import (
	"fmt"
	"github.com/yxxchange/richerLog/log"
	"gopkg.in/yaml.v3"
	"richerPipeline/pkg"
)

type IParser interface {
	Parse() (graph *WorkGraph, err error)
}

func NewGeneralParser(yamlBytes []byte) IParser {
	return &GeneralParser{yamlBytes: yamlBytes}
}

type GeneralParser struct {
	yamlBytes []byte
}

func (p *GeneralParser) Parse() (*WorkGraph, error) {
	var pipe RawPipeline
	err := yaml.Unmarshal(p.yamlBytes, &pipe)
	if err != nil {
		return nil, err
	}
	nodeMap := extractNode(pipe)
	edgeMap := extractEdge(pipe)
	graph, err := GenDAGraph(nodeMap, edgeMap)
	if err != nil {
		log.Errorf("生成DAG图失败: %v", err)
		return nil, err
	}
	return &WorkGraph{
		Metadata: pipe.Metadata,
		DAGraph:  graph,
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
	sorted, err := pkg.TopologicalSort(tmp)
	if err != nil {
		return WorkDAGraph{}, err
	}
	tmp = sorted.(*WorkDAGraph)
	res = *tmp
	return
}
