package database

import (
	"fmt"
	"github.com/yxxchange/richerPipeline/models"
)

type pipelineExecRepo struct{}

func NewPipeExecRepo() models.IPipelineExec {
	return pipelineExecRepo{}
}

var _ models.IPipelineExec = &pipelineExecRepo{}

func (p pipelineExecRepo) GetPipelineExec(id int64) (models.PipelineExec, error) {
	db := pipelineDB.Table(models.PipelineExec{}.TableName())
	var exec models.PipelineExec
	err := db.Where("id = ?", id).First(&exec).Error
	return exec, err
}

func (p pipelineExecRepo) CreatePipelineExec(exec models.PipelineExec) (int64, error) {
	db := pipelineDB.Table(models.PipelineExec{}.TableName())
	err := db.Create(&exec).Error
	return exec.Id, err
}

func (p pipelineExecRepo) FullUpdatePipelineExec(exec models.PipelineExec) error {
	if exec.Id <= 0 {
		return fmt.Errorf("id is required and valid")
	}
	db := pipelineDB.Table(models.PipelineExec{}.TableName())
	result := db.Where("id = ?", exec.Id).Updates(exec)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("no record updated")
	}
	return nil
}

func (p pipelineExecRepo) DeletePipelineExec(id int64) error {
	db := pipelineDB.Table(models.PipelineExec{}.TableName())
	return db.Where("id = ?", id).Delete(models.PipelineExec{}).Error
}

func (p pipelineExecRepo) UpdateColumn(id int64, column string, value interface{}) error {
	db := pipelineDB.Table(models.PipelineExec{}.TableName())
	result := db.Where("id = ?", id).Update(column, value)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("no record updated")
	}
	return nil
}
