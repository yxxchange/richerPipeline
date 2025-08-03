package client_pipe

import (
	"time"

	"github.com/yxxchange/pipefree/client_pipe/config"
	"github.com/yxxchange/pipefree/client_pipe/informers"
	"github.com/yxxchange/pipefree/client_pipe/typed"
)

// Interface 客户端接口
type Interface interface {
	// Typed clients
	NodeExecs() typed.NodeExecInterface

	// Informers
	Informers() informers.SharedInformerFactory
}

// Clientset 客户端集合
type Clientset struct {
	nodeExecs typed.NodeExecInterface
	informers informers.SharedInformerFactory
}

// ClientConfig 客户端配置
type ClientConfig struct {
	DefaultResyncPeriod time.Duration
}

// NewClientSet 创建客户端集合
func NewClientSet() Interface {
	return NewClientSetWithConfig(ClientConfig{
		DefaultResyncPeriod: 30 * time.Second,
	})
}

// NewClientSetWithConfig 使用配置创建客户端集合
func NewClientSetWithConfig(clientConfig ClientConfig) Interface {
	// 使用默认配置并合并用户配置
	defaultConfig := config.DefaultClientConfig()
	if clientConfig.DefaultResyncPeriod == 0 {
		clientConfig.DefaultResyncPeriod = defaultConfig.DefaultResyncPeriod
	}
	
	nodeExecClient := typed.NewNodeExecClientWithConfig(typed.ClientConfig{
		ServerURL: defaultConfig.ServerURL,
		Timeout:   defaultConfig.Timeout,
	})
	informerFactory := informers.NewSharedInformerFactory(nodeExecClient, clientConfig.DefaultResyncPeriod)

	return &Clientset{
		nodeExecs: nodeExecClient,
		informers: informerFactory,
	}
}

// NewForConfig 从配置创建客户端集合
func NewForConfig(config ClientConfig) (Interface, error) {
	return NewClientSetWithConfig(config), nil
}

// NewClientSetWithTestConfig 创建测试客户端集合
func NewClientSetWithTestConfig() Interface {
	testConfig := config.TestClientConfig()
	nodeExecClient := typed.NewNodeExecClientWithConfig(typed.ClientConfig{
		ServerURL: testConfig.ServerURL,
		Timeout:   testConfig.Timeout,
	})
	informerFactory := informers.NewSharedInformerFactory(nodeExecClient, testConfig.DefaultResyncPeriod)

	return &Clientset{
		nodeExecs: nodeExecClient,
		informers: informerFactory,
	}
}

// NodeExecs 获取节点执行客户端
func (c *Clientset) NodeExecs() typed.NodeExecInterface {
	return c.nodeExecs
}

// Informers 获取Informer工厂
func (c *Clientset) Informers() informers.SharedInformerFactory {
	return c.informers
}
