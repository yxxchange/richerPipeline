package infrastructure

import (
	"richerPipeline/infrastructure/database"
	"richerPipeline/models"
)

var PipelineCfgRepo models.IPipeCfg

func Init() {
	dbInterfaceInit()
}

func dbInterfaceInit() {
	database.Init()
	PipelineCfgRepo = database.NewPipeCfgRepo()
}
