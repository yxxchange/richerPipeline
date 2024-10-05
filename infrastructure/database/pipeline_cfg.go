package database

import (
	"fmt"
	"richerPipeline/models"
)

type pipelineCfgRepo struct {
}

func NewPipeCfgRepo() models.IPipelineCfg {
	return &pipelineCfgRepo{}
}

var _ models.IPipelineCfg = &pipelineCfgRepo{}

func (p *pipelineCfgRepo) GetPipelineCfg(id int64) (models.PipelineCfg, error) {
	db := pipelineDB.Table(models.PipelineCfg{}.TableName())
	var cfg models.PipelineCfg
	err := db.Where("id = ?", id).First(&cfg).Error
	return cfg, err
}

func (p *pipelineCfgRepo) CreatePipelineCfg(cfg *models.PipelineCfg) (int, error) {
	db := pipelineDB.Table(models.PipelineCfg{}.TableName())
	err := db.Create(cfg).Error
	return int(cfg.Id), err
}

func (p *pipelineCfgRepo) FullUpdatePipelineCfg(cfg *models.PipelineCfg) error {
	if cfg.Id <= 0 {
		return fmt.Errorf("id is required and valid")
	}
	db := pipelineDB.Table(models.PipelineCfg{}.TableName())
	result := db.Model(cfg).Where("id = ?", cfg.Id).Updates(cfg)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("no record updated")
	}
	return nil
}

func (p *pipelineCfgRepo) DeletePipelineCfg(id int64) error {
	db := pipelineDB.Table(models.PipelineCfg{}.TableName())
	return db.Where("id = ?", id).Delete(models.PipelineCfg{}).Error
}
