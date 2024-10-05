package models

import "time"

type IPipelineExec interface {
	CreatePipelineExec(exec *PipelineExec) (id int64, err error)
	GetPipelineExec(id int64) (exec PipelineExec, err error)
	FullUpdatePipelineExec(new PipelineExec) error
	DeletePipelineExec(id int64) error
	UpdateColumn(id int64, column string, value interface{}) error
}

type ExecStatus int

const (
	ExecStatusNotStart ExecStatus = iota + 1
	ExecStatusRunning
	ExecStatusPending
	ExecStatusSucceed
	ExecStatusFailed
)

type PipelineExec struct {
	Model
	PipelineCfgId int64      `gorm:"column:pipeline_cfg_id" json:"pipelineCfgId"`
	ExecStatus    ExecStatus `gorm:"column:exec_status" json:"execStatus"`
	ExecLog       string     `gorm:"column:exec_log" json:"execLog"`
	ExecSnapshot  string     `gorm:"column:exec_snapshot" json:"execSnapshot"`
	ExecStartTime time.Time  `gorm:"column:exec_start_time" json:"execStartTime"`
	ExecEndTime   time.Time  `gorm:"column:exec_end_time" json:"execEndTime"`
	Creator       string     `gorm:"column:creator" json:"creator"`
	CreatorID     int        `gorm:"column:creator_id" json:"creatorId"`
}

func (p PipelineExec) TableName() string {
	return "pipeline_exec"
}
