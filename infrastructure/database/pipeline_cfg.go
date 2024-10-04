package database

import "richerPipeline/models"

type pipelineCfgRepo struct {
}

var _ models.IPipeCfg = &pipelineCfgRepo{}

func (p *pipelineCfgRepo) GetPipeCfg(id int64) (models.PipelineCfg, error) {
	db := pipelineDB.Table(models.PipelineCfg{}.TableName())
	var cfg models.PipelineCfg
	err := db.Where("id = ?", id).First(&cfg).Error
	return cfg, err
}

func (p *pipelineCfgRepo) CreatePipeCfg(cfg *models.PipelineCfg) (int, error) {
	db := pipelineDB.Table(models.PipelineCfg{}.TableName())
	err := db.Create(cfg).Error
	return int(cfg.Id), err
}

func (p *pipelineCfgRepo) UpdatePipeCfg(cfg *models.PipelineCfg) error {
	db := pipelineDB.Table(models.PipelineCfg{}.TableName())
	return db.Model(cfg).Updates(cfg).Error
}

func (p *pipelineCfgRepo) DeletePipeCfg(id int64) error {
	db := pipelineDB.Table(models.PipelineCfg{}.TableName())
	return db.Where("id = ?", id).Delete(models.PipelineCfg{}).Error
}
