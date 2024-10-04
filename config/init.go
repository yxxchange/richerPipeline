package config

import (
	"github.com/spf13/viper"
)

var settings *Config

type Config struct {
	Path string
}

func WithPath(path string) *Config {
	if settings == nil {
		settings = &Config{}
	}
	settings.Path = path
	return settings
}

func (c *Config) Init() {
	if c.Path == "" {
		panic("config path is empty")
	}
	viper.SetConfigFile(c.Path)
	err := viper.ReadInConfig()
	if err != nil {
		panic(err)
	}
}

func Init() {
	if settings == nil {
		settings = &Config{}
	}
	settings.Init()
}
