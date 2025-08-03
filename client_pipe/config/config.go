package config

import (
	"time"
)

// ClientConfig 客户端配置
type ClientConfig struct {
	ServerURL           string        `yaml:"server_url" json:"server_url"`
	Timeout             time.Duration `yaml:"timeout" json:"timeout"`
	DefaultResyncPeriod time.Duration `yaml:"default_resync_period" json:"default_resync_period"`
}

// DefaultClientConfig 默认客户端配置
func DefaultClientConfig() ClientConfig {
	return ClientConfig{
		ServerURL:           "http://localhost:8080/api/v1",
		Timeout:             30 * time.Second,
		DefaultResyncPeriod: 30 * time.Second,
	}
}

// TestClientConfig 测试客户端配置
func TestClientConfig() ClientConfig {
	return ClientConfig{
		ServerURL:           "http://localhost:8081/api/v1",
		Timeout:             30 * time.Second,
		DefaultResyncPeriod: 30 * time.Second,
	}
}

// SimpleTestClientConfig 简单测试客户端配置
func SimpleTestClientConfig() ClientConfig {
	return ClientConfig{
		ServerURL:           "http://localhost:8082/api/v1",
		Timeout:             30 * time.Second,
		DefaultResyncPeriod: 30 * time.Second,
	}
}