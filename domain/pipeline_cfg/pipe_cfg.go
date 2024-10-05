package pipeline_cfg

import (
	"fmt"
	"github.com/yxxchange/richerLog/log"
	"richerPipeline/domain/graph"
	infra "richerPipeline/infrastructure"
	"richerPipeline/models"
	"richerPipeline/pkg"
)

type PipelineCfgHandler struct{}

func NewPipeCfgHandler() PipelineCfgHandler {
	return PipelineCfgHandler{}
}

func (h PipelineCfgHandler) CreatePipelineCfg(raw models.RawPipeline) error {
	parser, err := graph.NewParser(raw.PipelineVersion, models.PipelineType(raw.Metadata.Namespace))
	if err != nil {
		log.Errorf("pipeline解析器初始化失败: %v", err)
		return err
	}
	err = parser.Validate(raw)
	if err != nil {
		log.Errorf("pipeline数据校验失败: %v", err)
		return err
	}
	return h.createPipelineCfg(raw)
}

func (h PipelineCfgHandler) createPipelineCfg(raw models.RawPipeline) error {
	cfg, err := models.RawPipeline2PipelineCfg(raw)
	if err != nil {
		log.Errorf("数据模型转换失败: %v", err)
		return pkg.WrapError(pkg.ErrDataModelConvert, err)
	}
	id, err := infra.PipelineCfgRepo.CreatePipelineCfg(&cfg)
	if err != nil {
		log.Errorf("创建pipeline配置失败: %v", err)
		return pkg.WrapError(pkg.ErrDbOperation, err)
	}
	log.Infof("创建pipeline配置成功, id: %d", id)
	return nil
}

func (h PipelineCfgHandler) GetPipelineCfg(id int64) (models.PipelineCfg, error) {
	cfg, err := infra.PipelineCfgRepo.GetPipelineCfg(id)
	if err != nil {
		log.Errorf("获取pipeline配置失败: %v", err)
		return cfg, pkg.WrapError(pkg.ErrDbOperation, err)
	}
	return cfg, nil
}

func (h PipelineCfgHandler) UpdatePipelineCfg(id int64, raw models.RawPipeline) error {
	cfg, err := models.RawPipeline2PipelineCfg(raw)
	if err != nil {
		log.Errorf("数据模型转换失败: %v", err)
		return pkg.WrapError(pkg.ErrDataModelConvert, err)
	}
	err = infra.PipelineCfgRepo.FullUpdatePipelineCfg(&cfg)
	if err != nil {
		log.Errorf("更新pipeline配置失败: %v", err)
		return pkg.WrapError(pkg.ErrDbOperation, err)
	}
	log.Infof("更新pipeline配置成功, id: %d", id)
	return nil
}

func (h PipelineCfgHandler) DeletePipelineCfg(id int64) error {
	if id <= 0 {
		log.Errorf("id无效: %d", id)
		return pkg.WrapError(pkg.ErrDbOperation, fmt.Errorf("id无效: %d", id))
	}
	err := infra.PipelineCfgRepo.DeletePipelineCfg(id)
	if err != nil {
		log.Errorf("删除pipeline配置失败: %v", err)
		return pkg.WrapError(pkg.ErrDbOperation, err)
	}
	log.Infof("删除pipeline配置成功, id: %d", id)
	return nil
}
