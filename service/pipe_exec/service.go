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
