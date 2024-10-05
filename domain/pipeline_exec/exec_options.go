package pipeline_exec

import "github.com/yxxchange/richerPipeline/models"

type ExecOption func(exec *models.PipelineExec) error

func RunningWhenCreated() ExecOption {
	return func(exec *models.PipelineExec) error {
		exec.ExecStatus = models.ExecStatusRunning
		return nil
	}
}
