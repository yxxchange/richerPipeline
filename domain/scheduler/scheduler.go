package scheduler

type ExecutorType string

const (
	ExecutorTypeLocal  ExecutorType = "local"
	ExecutorTypeRemote ExecutorType = "remote"
)

// IExecutor 执行器接口
type IExecutor interface {
	Type() ExecutorType
	/*BuildWatchConn 建立执行器与分发器之间的通信,实现watch能力,实时监控任务变动情况，类似k8s的watch长连接*/
	BuildWatchConn() error
	HealthCheck() error
	Execute() error
}

type ExecutorRepo struct {
	repo
}

func (r *ExecutorRepo) Register(s IExecutor) {

}

type LocalExecutor struct {
	UniqueCode string
}

func (s LocalExecutor) Type() ExecutorType {
	return ExecutorTypeLocal
}
