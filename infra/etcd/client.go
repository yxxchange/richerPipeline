package etcd

import clientv3 "go.etcd.io/etcd/client/v3"

// GetClient 获取etcd客户端实例
func GetClient() *clientv3.Client {
	return cli
}