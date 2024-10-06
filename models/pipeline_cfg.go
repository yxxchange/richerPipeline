package models

const (
	DefaultPipeline = "default"
	CICDPipeline    = "ci_cd"
)

type IPipelineCfg interface {
	GetPipelineCfg(id int64) (PipelineCfg, error)
	CreatePipelineCfg(cfg *PipelineCfg) (int, error)
	FullUpdatePipelineCfg(cfg *PipelineCfg) error
	DeletePipelineCfg(id int64) error
}

type PipelineCfg struct {
	Model
	Name      string `json:"name" gorm:"name"`
	Version   string `json:"version" gorm:"version"`
	PipeType  string `json:"pipeType" gorm:"column:pipe_type"`
	CfgYaml   string `json:"cfgYaml" gorm:"column:cfg_yaml"`
	Creator   string `json:"creator" gorm:"column:creator"`
	CreatorID int    `json:"creatorId" gorm:"column:creator_id"`
}

func (p PipelineCfg) TableName() string {
	return "pipeline_cfg"
}
