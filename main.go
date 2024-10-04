package main

import (
	"flag"
	"github.com/yxxchange/richerLog/log"
	"richerPipeline/config"
	db "richerPipeline/infrastructure/database"
)

var (
	configPath string
)

func main() {
	flags := flag.NewFlagSet("richerPipeline", flag.ExitOnError)
	flags.StringVar(&configPath, "config", "./config.yml", "config file path")
	config.WithPath(configPath).Init()
	log.Init()
	db.Init()
	log.Infof("richerPipeline start")
}
