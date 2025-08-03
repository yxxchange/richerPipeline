package pipe_cfg

import (
	"context"
	"fmt"
	"github.com/yxxchange/pipefree/helper/log"
	"github.com/yxxchange/pipefree/infra/dal/dao"
	"github.com/yxxchange/pipefree/infra/dal/model"
	"github.com/yxxchange/pipefree/service/graph"
	"github.com/yxxchange/pipefree/service/pipe_perm"
)

const ErrorCode = 10001

type Service struct {
	pipeCfg dao.IPipeCfgDo
	nodeCfg dao.INodeCfgDo
	*dao.Query
	ctx context.Context
}

func NewService(ctx context.Context) *Service {
	return &Service{
		pipeCfg: dao.Q.PipeCfg.WithContext(ctx),
		nodeCfg: dao.Q.NodeCfg.WithContext(ctx),
		Query:   dao.Q,
		ctx:     ctx,
	}
}

func (s *Service) GetById(pipeId int64) (*model.PipeCfg, error) {
	pipeCfg, err := s.pipeCfg.Where(dao.PipeCfg.Id.Eq(pipeId)).First()
	if err != nil {
		return nil, err
	}
	return pipeCfg, nil
}

func (s *Service) Create(pipe *model.PipeCfg, nodes []*model.NodeCfg) (int64, error) {
	component := NewPipeComponent(pipe, nodes)
	if err := s.Validate(component); err != nil {
		log.Errorf("create pipe configuration failed, validate failed: %v", err)
		return 0, err
	}

	for name, vertex := range component.Graph.VertexMap {
		component.NodeMap[name].InDegree = vertex.InDegree
	}
	err := s.Transaction(func(tx *dao.Query) error {
		err := tx.PipeCfg.WithContext(s.ctx).Create(component.Pipe)
		if err != nil {
			return fmt.Errorf("create pipe configuration failed: %w", err)
		}
		for i := range component.NodeList {
			component.NodeList[i].PipeCfgId = pipe.Id
		}
		err = tx.NodeCfg.WithContext(s.ctx).CreateInBatches(component.NodeList, 100)
		if err != nil {
			return fmt.Errorf("create pipe configuration failed, create node configurations failed: %w", err)
		}
		return nil
	})

	if err != nil {
		return 0, err
	}
	return pipe.Id, nil
}

func (s *Service) Validate(component *PipeComponent) error {
	validateFlow := []func(*PipeComponent) error{
		s.nodeNameMustUnique,
		// s.validatePermission,
		s.validateGraph,
		s.mustBeDAG,
		s.validateNodeGraphConsistency,
	}
	for _, validate := range validateFlow {
		if err := validate(component); err != nil {
			log.Errorf("pipe configuration validation failed: %v", err)
			return err
		}
	}
	return nil
}

func (s *Service) nodeNameMustUnique(component *PipeComponent) error {
	if len(component.NodeList) > len(component.NodeMap) {
		return fmt.Errorf("duplicate node names found in node configuration")
	}
	return nil
}

func (s *Service) validatePermission(component *PipeComponent) error {
	return pipe_perm.NewService(s.ctx).PermissionBatchCheck(
		component.Pipe.Space,
		component.Namespaces,
		pipe_perm.PipePermissionBind,
	)
}

func (s *Service) validateGraph(component *PipeComponent) error {
	for name := range component.Graph.VertexMap {
		if _, exists := component.NodeMap[name]; !exists {
			return fmt.Errorf("create pipe configuration failed, node %s not found in node configurations", name)
		}
	}
	return nil
}

func (s *Service) mustBeDAG(component *PipeComponent) error {
	return graph.IsDAG(component.Graph)
}

func (s *Service) validateNodeGraphConsistency(component *PipeComponent) error {
	for name := range component.Graph.VertexMap {
		_, exists := component.NodeMap[name]
		if !exists {
			return fmt.Errorf("create pipe configuration failed, node %s not found in node configurations", name)
		}
	}
	return nil
}

func (s *Service) Update(pipeCfg *model.PipeCfg) error {
	if err := s.pipeCfg.Save(pipeCfg); err != nil {
		return err
	}
	return nil
}

func (s *Service) Delete(pipeId int64) error {
	_, err := s.pipeCfg.Where(dao.PipeCfg.Id.Eq(pipeId)).Delete()
	if err != nil {
		return err
	}
	log.Infof("Deleted pipe configuration with ID: %d", pipeId)
	return nil
}
