package graph

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/yxxchange/richerPipeline/models"
	"gopkg.in/yaml.v3"
	"io"
	"os"
	"testing"
)

func TestGenDAGraph_ValidInput(t *testing.T) {
	nodeMap := map[string]*models.NodeInfo{
		"node1": {Name: "node1"},
		"node2": {Name: "node2"},
		"node3": {Name: "node3"},
	}

	edgeMap := map[string][]string{
		"node1": {"node2"},
		"node2": {"node3"},
	}

	graph, err := GenDAGraph(nodeMap, edgeMap)

	assert.NoError(t, err)
	assert.NotNil(t, graph)
	assert.Equal(t, 3, len(graph.Map))
}

func TestGenDAGraph_EmptyInput(t *testing.T) {
	nodeMap := map[string]*models.NodeInfo{}
	edgeMap := map[string][]string{}

	graph, err := GenDAGraph(nodeMap, edgeMap)

	assert.Error(t, err)
	assert.NotNil(t, graph)
	assert.Equal(t, 0, len(graph.Map))
}

func TestGenDAGraph_NilInput(t *testing.T) {
	_, err := GenDAGraph(nil, nil)

	assert.Error(t, err)
}

func TestGenDAGraph_InvalidEdge(t *testing.T) {
	nodeMap := map[string]*models.NodeInfo{
		"node1": {Name: "node1"},
		"node2": {Name: "node2"},
	}

	edgeMap := map[string][]string{
		"node1": {"node3"},
	}

	graph, err := GenDAGraph(nodeMap, edgeMap)

	assert.Error(t, err)
	assert.Equal(t, WorkDAGraph{}, graph)
}

func TestGenDAGraph_MultipleEdges(t *testing.T) {
	nodeMap := map[string]*models.NodeInfo{
		"node1": {Name: "node1"},
		"node2": {Name: "node2"},
		"node3": {Name: "node3"},
		"node4": {Name: "node4"},
	}

	edgeMap := map[string][]string{
		"node1": {"node2", "node3"},
		"node2": {"node4"},
	}

	graph, err := GenDAGraph(nodeMap, edgeMap)

	fmt.Printf("slice: %v\n", graph.OutputTopologicalSlice())

	assert.NoError(t, err)
	assert.NotNil(t, graph)
	assert.Equal(t, 4, len(graph.Map))
}

func TestGenDAGraph_SingleNode(t *testing.T) {
	nodeMap := map[string]*models.NodeInfo{
		"node1": {Name: "node1"},
	}

	edgeMap := map[string][]string{}

	graph, err := GenDAGraph(nodeMap, edgeMap)

	assert.NoError(t, err)
	assert.NotNil(t, graph)
	assert.Equal(t, 1, len(graph.Map))
}

func TestGenDAGraph_CircularDependency(t *testing.T) {
	nodeMap := map[string]*models.NodeInfo{
		"node1": {Name: "node1"},
		"node2": {Name: "node2"},
	}

	edgeMap := map[string][]string{
		"node1": {"node2"},
		"node2": {"node1"},
	}

	graph, err := GenDAGraph(nodeMap, edgeMap)

	assert.Error(t, err)
	assert.Equal(t, WorkDAGraph{}, graph)
}

func TestGenDAGraph_ComplexGraph(t *testing.T) {
	nodeMap := map[string]*models.NodeInfo{
		"node1": {Name: "node1"},
		"node2": {Name: "node2"},
		"node3": {Name: "node3"},
		"node4": {Name: "node4"},
		"node5": {Name: "node5"},
		"node6": {Name: "node6"},
		"node7": {Name: "node7"},
	}

	edgeMap := map[string][]string{
		"node1": {"node2", "node3"},
		"node2": {"node4", "node5"},
		"node3": {"node6"},
		"node4": {"node7"},
		"node5": {"node7"},
		"node6": {"node7"},
	}

	graph, err := GenDAGraph(nodeMap, edgeMap)

	fmt.Printf("slice: %v\n", graph.OutputTopologicalSlice())

	assert.NoError(t, err)
	assert.NotNil(t, graph)
	assert.Equal(t, 7, len(graph.Map))
}

func TestGenDAGraph_LargeGraphWithHighInDegree(t *testing.T) {
	nodeMap := map[string]*models.NodeInfo{
		"node1":  {Name: "node1"},
		"node2":  {Name: "node2"},
		"node3":  {Name: "node3"},
		"node4":  {Name: "node4"},
		"node5":  {Name: "node5"},
		"node6":  {Name: "node6"},
		"node7":  {Name: "node7"},
		"node8":  {Name: "node8"},
		"node9":  {Name: "node9"},
		"node10": {Name: "node10"},
		"node11": {Name: "node11"},
		"node12": {Name: "node12"},
		"node13": {Name: "node13"},
		"node14": {Name: "node14"},
		"node15": {Name: "node15"},
	}

	edgeMap := map[string][]string{
		"node1":  {"node2", "node3", "node4", "node5", "node6"},
		"node2":  {"node7", "node8", "node9", "node10", "node11"},
		"node3":  {"node12", "node13", "node14", "node15"},
		"node4":  {"node7", "node8", "node9", "node10", "node11"},
		"node5":  {"node12", "node13", "node14", "node15"},
		"node6":  {"node7", "node8", "node9", "node10", "node11"},
		"node7":  {"node12", "node13", "node14", "node15"},
		"node8":  {"node12", "node13", "node14", "node15"},
		"node9":  {"node12", "node13", "node14", "node15"},
		"node10": {"node12", "node13", "node14", "node15"},
		"node11": {"node12", "node13", "node14", "node15"},
	}

	graph, err := GenDAGraph(nodeMap, edgeMap)

	fmt.Printf("slice: %v\n", graph.OutputTopologicalSlice())

	assert.NoError(t, err)
	assert.NotNil(t, graph)
	assert.Equal(t, 15, len(graph.Map))
}

func TestGenDAGraph_OrphanNode(t *testing.T) {
	nodeMap := map[string]*models.NodeInfo{
		"node1": {Name: "node1"},
		"node2": {Name: "node2"},
		"node3": {Name: "node3"},
		"node4": {Name: "node4"}, // This is the orphan node
	}

	edgeMap := map[string][]string{
		"node1": {"node2"},
		"node2": {"node3"},
		// node4 is not connected to any other nodes
	}

	graph, err := GenDAGraph(nodeMap, edgeMap)

	fmt.Printf("slice: %v\n", graph.OutputTopologicalSlice())

	assert.NoError(t, err)
	// assert.Equal(t, WorkDAGraph{}, graph)
	assert.Equal(t, 4, len(graph.Map))
}

func TestGenDAGraph_MultipleStartNodes(t *testing.T) {
	nodeMap := map[string]*models.NodeInfo{
		"node1": {Name: "node1"},
		"node2": {Name: "node2"},
		"node3": {Name: "node3"},
		"node4": {Name: "node4"},
		"node5": {Name: "node5"},
	}

	edgeMap := map[string][]string{
		"node1": {"node3"},
		"node2": {"node4"},
		"node3": {"node5"},
		"node4": {"node5"},
	}

	graph, err := GenDAGraph(nodeMap, edgeMap)

	fmt.Printf("slice: %v\n", graph.OutputTopologicalSlice())

	assert.NoError(t, err)
	assert.NotNil(t, graph)
	assert.Equal(t, 5, len(graph.Map))
}

func TestGenDAGraph_EmptyEdgeMap(t *testing.T) {
	nodeMap := map[string]*models.NodeInfo{
		"node1": {Name: "node1"},
		"node2": {Name: "node2"},
		"node3": {Name: "node3"},
	}

	edgeMap := map[string][]string{}

	graph, err := GenDAGraph(nodeMap, edgeMap)

	fmt.Printf("slice: %v\n", graph.OutputTopologicalSlice())
	assert.NoError(t, err)
	assert.NotNil(t, graph)
	assert.Equal(t, 3, len(graph.Map))
}

func TestGenDAGraph_InvalidTargetNode(t *testing.T) {
	nodeMap := map[string]*models.NodeInfo{
		"node1": {Name: "node1"},
		"node2": {Name: "node2"},
	}

	edgeMap := map[string][]string{
		"node1": {"node3"},
	}

	graph, err := GenDAGraph(nodeMap, edgeMap)

	assert.Error(t, err)
	assert.Equal(t, WorkDAGraph{}, graph)
}

func TestGenDAGraph_InvalidSourceNode(t *testing.T) {
	nodeMap := map[string]*models.NodeInfo{
		"node1": {Name: "node1"},
		"node2": {Name: "node2"},
	}

	edgeMap := map[string][]string{
		"node3": {"node2"},
	}

	graph, err := GenDAGraph(nodeMap, edgeMap)

	fmt.Printf("slice: %v\n", graph.OutputTopologicalSlice())
	assert.Error(t, err)
	assert.Equal(t, WorkDAGraph{}, graph)
}

func TestGeneralParser_Parse(t *testing.T) {
	filePath := "../../example.yml"
	r, err := os.Open(filePath)
	if err != nil {
		t.Fatal(err)
	}
	b, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	var raw models.RawPipeline
	err = yaml.Unmarshal(b, &raw)
	if err != nil {
		t.Fatal(err)
	}
	parser, err := NewParser(raw.PipelineVersion, models.PipelineType(raw.Metadata.Namespace))
	graph, err := parser.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("graph: %v\n", graph)
}
