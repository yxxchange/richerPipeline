package pipeline_exec

import (
	"github.com/yxxchange/richerLog/log"
	infra "github.com/yxxchange/richerPipeline/infrastructure"
	"github.com/yxxchange/richerPipeline/models"
	"github.com/yxxchange/richerPipeline/pkg"
)

type PipelineExecHandler struct{}

func NewPipelineExecHandler() PipelineExecHandler {
	return PipelineExecHandler{}
}

type ExecOption func(exec *models.PipelineExec) error

func (h PipelineExecHandler) CreatePipelineExec(cfg models.PipelineCfg, opt ...ExecOption) error {
	exec, err := models.PipelineCfg2Exec(cfg)
	if err != nil {
		log.Errorf("转换PipelineCfg到PipelineExec失败: %v", err)
		return pkg.WrapError(pkg.ErrDataModelConvert, err)
	}
	for _, o := range opt {
		err = o(&exec)
		if err != nil {
			return err
		}
	}
	_, err = infra.PipelineExecRepo.CreatePipelineExec(exec)
	if err != nil {
		log.Errorf("创建pipeline执行快照失败: %v", err)
		return pkg.WrapError(pkg.ErrDbOperation, err)
	}
	return nil
}

func RunningWhenCreated() ExecOption {
	return func(exec *models.PipelineExec) error {
		exec.ExecStatus = models.ExecStatusRunning
		return nil
	}
}
