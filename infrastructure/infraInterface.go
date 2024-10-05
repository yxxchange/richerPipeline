package infrastructure

import (
	"github.com/yxxchange/richerPipeline/infrastructure/database"
	"github.com/yxxchange/richerPipeline/models"
)

var PipelineCfgRepo models.IPipelineCfg
var PipelineExecRepo models.IPipelineExec

func Init() {
	dbInterfaceInit()
}

func dbInterfaceInit() {
	database.Init()
	PipelineCfgRepo = database.NewPipeCfgRepo()
	PipelineExecRepo = database.NewPipeExecRepo()
}
