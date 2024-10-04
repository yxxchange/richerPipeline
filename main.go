package main

import (
	"flag"
	"github.com/yxxchange/richerLog/log"
	"os"
	"richerPipeline/config"
	"richerPipeline/infrastructure"
)

var (
	configPath string
)

func main() {
	// step 1: init config
	flags := flag.NewFlagSet("richerPipeline", flag.ExitOnError)
	flags.StringVar(&configPath, "config", "./config.yml", "config file path")
	err := flags.Parse(os.Args[1:])
	if err != nil {
		panic(err)
	}
	config.WithPath(configPath).Init()

	// step 2: init log
	log.UseDefault()
	log.Infof("log init")

	// step 3: init infrastructure
	infrastructure.Init()
	log.Infof("infrastructure init")

	// step 4: init domain
	// domain.Init()
}
