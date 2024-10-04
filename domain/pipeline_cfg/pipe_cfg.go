package pipeline_cfg

import (
	"github.com/yxxchange/richerLog/log"
	infra "richerPipeline/infrastructure"
	"richerPipeline/models"
	"richerPipeline/pkg"
)

type PipelineCfgHandler struct{}

func NewPipeCfgHandler() PipelineCfgHandler {
	return PipelineCfgHandler{}
}

func (h PipelineCfgHandler) CreatePipelineCfg(raw models.RawPipeline) error {
	cfg, err := models.RawPipeline2PipelineCfg(raw)
	if err != nil {
		log.Errorf("数据模型转换失败: %v", err)
		return pkg.ErrDataModelConvert
	}
	id, err := infra.PipelineCfgRepo.CreatePipeCfg(&cfg)
	if err != nil {
		log.Errorf("创建pipeline配置失败: %v", err)
		return pkg.ErrDbOperation
	}
	log.Infof("创建pipeline配置成功, id: %d", id)
	return nil
}
