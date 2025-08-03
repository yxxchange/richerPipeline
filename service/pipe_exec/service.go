package pipe_exec

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/yxxchange/pipefree/helper/log"
	"github.com/yxxchange/pipefree/infra/dal/dao"
	"github.com/yxxchange/pipefree/infra/dal/model"
	"github.com/yxxchange/pipefree/infra/etcd"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// NodeExecList 节点执行列表
type NodeExecList struct {
	Items           []*model.NodeExec `json:"items"`
	ResourceVersion string            `json:"resourceVersion"`
	Continue        string            `json:"continue,omitempty"`
}

const ErrorCode = 10002

type Service struct {
	pipeExec dao.IPipeExecDo
	nodeExec dao.INodeExecDo
	pipeCfg  dao.IPipeCfgDo
	nodeCfg  dao.INodeCfgDo
	*dao.Query
	ctx context.Context
}

func NewService(ctx context.Context) *Service {
	return &Service{
		pipeExec: dao.Q.PipeExec.WithContext(ctx),
		nodeExec: dao.Q.NodeExec.WithContext(ctx),
		pipeCfg:  dao.Q.PipeCfg.WithContext(ctx),
		nodeCfg:  dao.Q.NodeCfg.WithContext(ctx),
		Query:    dao.Q,
		ctx:      ctx,
	}
}

func (s *Service) Run(pipeId int64) error {
	pipeCfg, err := s.pipeCfg.Where(dao.PipeCfg.Id.Eq(pipeId)).First()
	if err != nil {
		log.Errorf("run pipe execution failed, get pipe configuration failed: %v", err)
		return err
	}
	nodeCfgList, err := s.nodeCfg.Where(dao.NodeCfg.PipeCfgId.Eq(pipeId)).Find()
	if err != nil {
		log.Errorf("run pipe execution failed, get node configurations failed: %v", err)
	}
	err = s.Query.Transaction(func(tx *dao.Query) error {
		return run(tx, s.ctx, pipeCfg, nodeCfgList)
	})
	if err != nil {
		log.Errorf("run pipe execution failed, transaction failed: %v", err)
		return err
	}
	return nil
}

func (s *Service) List(namespace, kind string, limit int64) (*NodeExecList, error) {
	// 从 etcd 读取数据
	etcdClient := etcd.GetClient()
	
	// 构建 etcd key prefix
	keyPrefix := s.buildKeyPrefix(namespace, kind)
	
	resp, err := etcdClient.Get(s.ctx, keyPrefix, clientv3.WithPrefix())
	if err != nil {
		return nil, fmt.Errorf("failed to list from etcd: %w", err)
	}
	
	var items []*model.NodeExec
	var maxRevision int64
	
	for _, kv := range resp.Kvs {
		var nodeExec model.NodeExec
		if err := json.Unmarshal(kv.Value, &nodeExec); err != nil {
			log.Warnf("failed to unmarshal node exec from key %s: %v", string(kv.Key), err)
			continue
		}
		
		// 过滤条件
		if namespace != "" && nodeExec.Namespace != namespace {
			continue
		}
		if kind != "" && nodeExec.Kind != kind {
			continue
		}
		
		items = append(items, &nodeExec)
		if kv.ModRevision > maxRevision {
			maxRevision = kv.ModRevision
		}
	}
	
	// 应用 limit
	if limit > 0 && int64(len(items)) > limit {
		items = items[:limit]
	}
	
	resourceVersion := strconv.FormatInt(maxRevision, 10)
	if maxRevision == 0 {
		resourceVersion = strconv.FormatInt(resp.Header.Revision, 10)
	}
	
	return &NodeExecList{
		Items:           items,
		ResourceVersion: resourceVersion,
	}, nil
}

func (s *Service) Get(namespace, kind string, id int64) (*model.NodeExec, error) {
	// 从 etcd 读取数据
	etcdClient := etcd.GetClient()
	
	key := s.buildObjectKey(namespace, kind, id)
	resp, err := etcdClient.Get(s.ctx, key)
	if err != nil {
		return nil, fmt.Errorf("failed to get from etcd: %w", err)
	}
	
	if len(resp.Kvs) == 0 {
		return nil, fmt.Errorf("node exec not found: %s/%s/%d", namespace, kind, id)
	}
	
	var nodeExec model.NodeExec
	if err := json.Unmarshal(resp.Kvs[0].Value, &nodeExec); err != nil {
		return nil, fmt.Errorf("failed to unmarshal node exec: %w", err)
	}
	
	return &nodeExec, nil
}

// Update 更新节点执行状态
func (s *Service) Update(nodeExec *model.NodeExec) (*model.NodeExec, error) {
	// 1. 更新数据库
	_, err := s.nodeExec.Where(dao.NodeExec.Id.Eq(nodeExec.Id)).Updates(nodeExec)
	if err != nil {
		log.Errorf("update node exec in db failed: %v", err)
		return nil, err
	}
	
	// 2. 更新 etcd
	key := KeyGen(nodeExec)
	value, err := ValueGen(nodeExec)
	if err != nil {
		log.Errorf("generate value failed: %v", err)
		return nil, err
	}
	
	err = etcd.Put(s.ctx, key, value)
	if err != nil {
		log.Errorf("update node exec in etcd failed: %v", err)
		return nil, err
	}
	
	// 3. 检查是否进入结束状态，如果是则触发流水线逻辑
	if s.isTerminalState(nodeExec.Phase.Phase) {
		go s.handleNodeCompletion(nodeExec)
	}
	
	return nodeExec, nil
}

// Delete 删除节点执行
func (s *Service) Delete(namespace, kind string, id int64) error {
	// 1. 从数据库删除
	_, err := s.nodeExec.Where(
		dao.NodeExec.Id.Eq(id),
		dao.NodeExec.Namespace.Eq(namespace),
		dao.NodeExec.Kind.Eq(kind),
	).Delete()
	if err != nil {
		log.Errorf("delete node exec from db failed: %v", err)
		return err
	}
	
	// 2. 从 etcd 删除
	key := s.buildObjectKey(namespace, kind, id)
	_, err = etcd.GetClient().Delete(s.ctx, key)
	if err != nil {
		log.Errorf("delete node exec from etcd failed: %v", err)
		return err
	}
	
	return nil
}

// isTerminalState 判断是否为结束状态
func (s *Service) isTerminalState(phase string) bool {
	return phase == model.NodePhaseSucceeded || phase == model.NodePhaseFailed
}

// handleNodeCompletion 处理节点完成后的逻辑
func (s *Service) handleNodeCompletion(completedNode *model.NodeExec) {
	log.Infof("Node %s completed with status %s", completedNode.Name, completedNode.Phase.Phase)
	
	// 1. 从 etcd 中删除运行时数据
	key := KeyGen(completedNode)
	_, err := etcd.GetClient().Delete(context.Background(), key)
	if err != nil {
		log.Errorf("failed to delete runtime data from etcd: %v", err)
	}
	
	// 2. 查找下游节点并更新入度
	if completedNode.Phase.Phase == model.NodePhaseSucceeded {
		s.triggerDownstreamNodes(completedNode)
	}
}

// triggerDownstreamNodes 触发下游节点
func (s *Service) triggerDownstreamNodes(completedNode *model.NodeExec) {
	// 查找同一流水线下的所有节点配置
	nodeCfgs, err := s.nodeCfg.Where(dao.NodeCfg.PipeCfgId.Eq(completedNode.PipeCfgId)).Find()
	if err != nil {
		log.Errorf("failed to get node configs: %v", err)
		return
	}
	
	// 构建依赖关系图
	dependencyMap := s.buildDependencyMap(nodeCfgs)
	
	// 查找依赖当前节点的下游节点
	downstreamNodes := dependencyMap[completedNode.Name]
	
	for _, downstreamName := range downstreamNodes {
		// 获取下游节点的执行实例
		downstreamExec, err := s.nodeExec.Where(
			dao.NodeExec.Name.Eq(downstreamName),
			dao.NodeExec.PipeExecId.Eq(completedNode.PipeExecId),
		).First()
		if err != nil {
			log.Errorf("failed to get downstream node %s: %v", downstreamName, err)
			continue
		}
		
		// 使用事务更新入度
		err = s.updateDownstreamNodeWithTransaction(downstreamExec, downstreamName)
		if err != nil {
			log.Errorf("failed to update downstream node %s: %v", downstreamName, err)
			continue
		}
	}
	
	// 检查流水线是否完成
	s.checkPipelineCompletion(completedNode.PipeExecId)
}

// buildDependencyMap 构建依赖关系图
func (s *Service) buildDependencyMap(nodeCfgs []*model.NodeCfg) map[string][]string {
	if len(nodeCfgs) == 0 {
		return make(map[string][]string)
	}
	
	// 获取流水线配置
	pipeCfg, err := s.pipeCfg.Where(dao.PipeCfg.Id.Eq(nodeCfgs[0].PipeCfgId)).First()
	if err != nil {
		log.Errorf("failed to get pipe config: %v", err)
		return make(map[string][]string)
	}
	
	dependencyMap := make(map[string][]string)
	
	// 从流水线图结构中构建依赖关系
	if pipeCfg.Graph != nil && len(pipeCfg.Graph.Edges) > 0 {
		for _, edge := range pipeCfg.Graph.Edges {
			// edge.From -> edge.To
			if dependencyMap[edge.From] == nil {
				dependencyMap[edge.From] = make([]string, 0)
			}
			dependencyMap[edge.From] = append(dependencyMap[edge.From], edge.To)
		}
	} else {
		// 如果没有图结构，使用简单的顺序执行（仅作为示例）
		for i := 0; i < len(nodeCfgs)-1; i++ {
			current := nodeCfgs[i].Name
			next := nodeCfgs[i+1].Name
			dependencyMap[current] = []string{next}
		}
	}
	
	return dependencyMap
}

// updateDownstreamNodeWithTransaction 使用事务更新下游节点
func (s *Service) updateDownstreamNodeWithTransaction(downstreamExec *model.NodeExec, downstreamName string) error {
	return s.Query.Transaction(func(tx *dao.Query) error {
		// 减少入度
		downstreamExec.InDegree--
		
		// 如果入度为0，则设置为Ready状态
		if downstreamExec.InDegree == 0 {
			downstreamExec.Phase.Phase = model.NodePhaseReady
		}
		
		// 更新数据库
		_, err := tx.NodeExec.WithContext(s.ctx).Where(dao.NodeExec.Id.Eq(downstreamExec.Id)).Updates(downstreamExec)
		if err != nil {
			return fmt.Errorf("failed to update node in db: %w", err)
		}
		
		// 如果入度为0，写入etcd触发执行
		if downstreamExec.InDegree == 0 {
			key := KeyGen(downstreamExec)
			value, err := ValueGen(downstreamExec)
			if err != nil {
				return fmt.Errorf("failed to generate value: %w", err)
			}
			
			err = etcd.Put(s.ctx, key, value)
			if err != nil {
				return fmt.Errorf("failed to put to etcd: %w", err)
			}
			
			log.Infof("✅ Triggered downstream node: %s (InDegree: 0 -> Ready)", downstreamName)
		} else {
			log.Infof("⏳ Downstream node %s waiting (InDegree: %d)", downstreamName, downstreamExec.InDegree)
		}
		
		return nil
	})
}

// checkPipelineCompletion 检查流水线是否完成
func (s *Service) checkPipelineCompletion(pipeExecId int64) {
	// 查找流水线下所有节点
	nodes, err := s.nodeExec.Where(dao.NodeExec.PipeExecId.Eq(pipeExecId)).Find()
	if err != nil {
		log.Errorf("failed to get pipeline nodes: %v", err)
		return
	}
	
	allCompleted := true
	successCount := 0
	failedCount := 0
	
	for _, node := range nodes {
		switch node.Phase.Phase {
		case model.NodePhaseSucceeded:
			successCount++
		case model.NodePhaseFailed:
			failedCount++
			allCompleted = false
		default:
			allCompleted = false
		}
	}
	
	if allCompleted {
		if failedCount > 0 {
			log.Infof("🔴 Pipeline %d completed with failures: %d succeeded, %d failed", pipeExecId, successCount, failedCount)
			s.updatePipelineStatus(pipeExecId, "Failed")
		} else {
			log.Infof("🟢 Pipeline %d completed successfully: %d nodes succeeded", pipeExecId, successCount)
			s.updatePipelineStatus(pipeExecId, "Succeeded")
		}
	}
}

// updatePipelineStatus 更新流水线状态
func (s *Service) updatePipelineStatus(pipeExecId int64, status string) {
	// 获取流水线执行实例
	pipeExec, err := s.pipeExec.Where(dao.PipeExec.Id.Eq(pipeExecId)).First()
	if err != nil {
		log.Errorf("failed to get pipe exec: %v", err)
		return
	}
	
	// 更新状态
	var state int
	switch status {
	case "Succeeded":
		state = model.PipeExecStateSuccess
	case "Failed":
		state = model.PipeExecStateFailed
	default:
		state = model.PipeExecStateRunning
	}
	
	pipeExec.State = state
	_, err = s.pipeExec.Where(dao.PipeExec.Id.Eq(pipeExecId)).Updates(pipeExec)
	if err != nil {
		log.Errorf("failed to update pipeline status: %v", err)
		return
	}
	
	log.Infof("🏁 Pipeline %d status updated to: %s (state=%d)", pipeExecId, status, state)
}
func run(tx *dao.Query, ctx context.Context, pipeCfg *model.PipeCfg, nodeCfgList []*model.NodeCfg) error {
	pipeExec := model.NewPipeExec(pipeCfg)
	err := tx.PipeExec.WithContext(ctx).Create(pipeExec)
	if err != nil {
		log.Errorf("run pipe execution failed, create pipe execution failed: %v", err)
		return err
	}
	nodeExecList := make([]*model.NodeExec, 0, len(nodeCfgList))
	for _, nodeCfg := range nodeCfgList {
		nodeExec := model.NewNodeExec(nodeCfg, pipeExec)
		nodeExecList = append(nodeExecList, nodeExec)
	}
	err = tx.NodeExec.WithContext(ctx).Create(nodeExecList...)
	if err != nil {
		log.Errorf("run pipe execution failed, create node executions failed: %v", err)
		return err
	}
	initialNodes := make([]*model.NodeExec, 0)
	for _, nodeExec := range nodeExecList {
		if nodeExec.InDegree == 0 {
			initialNodes = append(initialNodes, nodeExec)
		}
	}
	kv := make(map[string]string, len(initialNodes))
	for _, nodeExec := range initialNodes {
		if val, e := ValueGen(nodeExec); e != nil {
			log.Errorf("run pipe execution failed, generate value for node %s failed: %v", nodeExec.Name, e)
			return e
		} else {
			kv[KeyGen(nodeExec)] = val
		}
	}
	err = etcd.TransactionPut(ctx, kv)
	if err != nil {
		log.Errorf("run pipe execution failed, transaction put to etcd failed: %v", err)
		return err
	}
	return nil
}

// buildKeyPrefix 构建 etcd key 前缀
func (s *Service) buildKeyPrefix(namespace, kind string) string {
	if namespace != "" && kind != "" {
		return fmt.Sprintf("/namespace/%s/kind/%s/", namespace, kind)
	} else if namespace != "" {
		return fmt.Sprintf("/namespace/%s/", namespace)
	} else if kind != "" {
		return fmt.Sprintf("/namespace/*/kind/%s/", kind)
	}
	return "/namespace/"
}

// buildObjectKey 构建对象 key（使用 KeyGen 生成完整 key）
func (s *Service) buildObjectKey(namespace, kind string, id int64) string {
	// 使用与 KeyGen 相同的格式，但需要 version 信息
	// 这里简化为直接查找，实际应该传入 version
	return fmt.Sprintf("/namespace/%s/kind/%s/version/*/node_exec/%d", namespace, kind, id)
}
