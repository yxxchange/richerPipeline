package config

import (
	"github.com/spf13/viper"
	_ "github.com/spf13/viper"
)

func Init() {
	viper.SetConfigFile("./config.yml")
	err := viper.ReadInConfig()
	if err != nil {
		panic(err)
	}
}
