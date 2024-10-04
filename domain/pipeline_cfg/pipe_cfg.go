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

func (h PipelineCfgHandler) GetPipelineCfg(id int64) (models.PipelineCfg, error) {
	cfg, err := infra.PipelineCfgRepo.GetPipeCfg(id)
	if err != nil {
		log.Errorf("获取pipeline配置失败: %v", err)
		return cfg, pkg.ErrDbOperation
	}
	return cfg, nil
}

func (h PipelineCfgHandler) UpdatePipelineCfg(id int64, raw models.RawPipeline) error {
	cfg, err := models.RawPipeline2PipelineCfg(raw)
	if err != nil {
		log.Errorf("数据模型转换失败: %v", err)
		return pkg.ErrDataModelConvert
	}
	err = infra.PipelineCfgRepo.FullUpdatePipeCfg(&cfg)
	if err != nil {
		log.Errorf("更新pipeline配置失败: %v", err)
		return pkg.ErrDbOperation
	}
	log.Infof("更新pipeline配置成功, id: %d", id)
	return nil
}

func (h PipelineCfgHandler) DeletePipelineCfg(id int64) error {
	if id <= 0 {
		log.Errorf("id无效: %d", id)
		return pkg.ErrDbOperation
	}
	err := infra.PipelineCfgRepo.DeletePipeCfg(id)
	if err != nil {
		log.Errorf("删除pipeline配置失败: %v", err)
		return pkg.ErrDbOperation
	}
	log.Infof("删除pipeline配置成功, id: %d", id)
	return nil
}
