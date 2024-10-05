package infrastructure

import (
	"richerPipeline/infrastructure/database"
	"richerPipeline/models"
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
