package database

import (
	"fmt"
	"richerPipeline/models"
)

type pipelineCfgRepo struct {
}

func NewPipeCfgRepo() models.IPipeCfg {
	return &pipelineCfgRepo{}
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

func (p *pipelineCfgRepo) FullUpdatePipeCfg(cfg *models.PipelineCfg) error {
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

func (p *pipelineCfgRepo) DeletePipeCfg(id int64) error {
	db := pipelineDB.Table(models.PipelineCfg{}.TableName())
	return db.Where("id = ?", id).Delete(models.PipelineCfg{}).Error
}
