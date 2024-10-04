package models

type PipelineType string

const (
	GeneralPipeline PipelineType = "general"
	CICDPipeline    PipelineType = "ci_cd"
)

type IPipeCfg interface {
	GetPipeCfg(id int64) (PipelineCfg, error)
	CreatePipeCfg(cfg *PipelineCfg) (int, error)
	UpdatePipeCfg(cfg *PipelineCfg) error
	DeletePipeCfg(id int64) error
}

type PipelineCfg struct {
	Model
	Name      string       `json:"name" gorm:"name"`
	Version   string       `json:"version" gorm:"version"`
	PipeType  PipelineType `json:"pipeType" gorm:"column:pipe_type"`
	CfgYaml   string       `json:"cfgYaml" gorm:"column:cfg_yaml"`
	Creator   string       `json:"creator" gorm:"column:creator"`
	CreatorID int          `json:"creatorId" gorm:"column:creator_id"`
}

func (p PipelineCfg) TableName() string {
	return "pipeline_cfg"
}
