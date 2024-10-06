package models

type PipelineType string

const (
	DefaultPipeline PipelineType = "default"
	CICDPipeline    PipelineType = "ci_cd"
)

type IPipelineCfg interface {
	GetPipelineCfg(id int64) (PipelineCfg, error)
	CreatePipelineCfg(cfg *PipelineCfg) (int, error)
	FullUpdatePipelineCfg(cfg *PipelineCfg) error
	DeletePipelineCfg(id int64) error
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

type RawPipeline struct {
	PipelineVersion string   `yaml:"pipelineVersion"`
	Metadata        Metadata `yaml:"metadata"`
	Graph           RawGraph `yaml:"graph"`
}

type Metadata struct {
	Name      string `yaml:"name"`
	Namespace string `yaml:"namespace"`
}

type RawGraph struct {
	Nodes []NodeInfo `yaml:"nodes"`
	Edges []EdgeInfo `yaml:"edges"`
}

type NodeInfo struct {
	Name   string  `yaml:"name"`
	Ctx    Context `yaml:"ctx"`
	Config Config  `yaml:"config"`
	Status Status  `yaml:"status"`
}

type Context struct {
	Input Input `yaml:"input"`
}

type Input struct {
	Worker    string `yaml:"worker"`
	JsonParam string `yaml:"jsonParam"`
}

type Config struct {
	Retry           int    `yaml:"retry"`
	Timeout         int    `yaml:"timeout"`
	TimeoutPolicy   string `yaml:"timeoutPolicy"`
	SchedulerPolicy string `yaml:"schedulerPolicy"`
}

type Status struct {
	State       string `yaml:"state"`
	StartTime   uint64 `yaml:"startTime"`
	EndTime     uint64 `yaml:"endTime"`
	Duration    uint64 `yaml:"duration"`
	ErrMsg      string `yaml:"errMsg"`
	Data        string `yaml:"data"`
	ExecutorUid string `yaml:"-"`
}

type EdgeInfo struct {
	Source string `yaml:"source"`
	Target string `yaml:"target"`
}
