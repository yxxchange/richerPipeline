package scheduler

import "github.com/yxxchange/richerPipeline/models"

type ExecutorType string

const (
	ExecutorTypeLocal  ExecutorType = "local"
	ExecutorTypeRemote ExecutorType = "remote"
)

// Receiver 执行器接口
type Receiver interface {
	Type() ExecutorType

	Receive(event models.Event) error

	Brief() ExecutorBrief
}

type ExecutorRepo struct {
}

func (r *ExecutorRepo) Register(s Receiver) {

}

type ExecutorBrief struct {
	WorkEngine      string
	PipelineVersion string
	PipelineType    string
}

type LocalExecutor struct {
	UniqueCode string
}

func (s LocalExecutor) Type() ExecutorType {
	return ExecutorTypeLocal
}
