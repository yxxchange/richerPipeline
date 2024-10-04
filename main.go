package main

import (
	"flag"
	"github.com/yxxchange/richerLog/log"
	"os"
	"richerPipeline/config"
	db "richerPipeline/infrastructure/database"
)

var (
	configPath string
)

func main() {
	flags := flag.NewFlagSet("richerPipeline", flag.ExitOnError)
	flags.StringVar(&configPath, "config", "./config.yml", "config file path")
	err := flags.Parse(os.Args[1:])
	if err != nil {
		panic(err)
	}
	config.WithPath(configPath).Init()
	log.UseDefault()
	db.Init()
	log.Infof("richerPipeline start")
}
