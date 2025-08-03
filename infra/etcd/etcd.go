package etcd

import (
	"context"
	"fmt"
	"github.com/spf13/viper"
	"github.com/yxxchange/pipefree/helper/log"
	clientv3 "go.etcd.io/etcd/client/v3"
	"time"
)

var cli *clientv3.Client

type Response struct {
	Key      []byte `json:"key"`
	Value    []byte `json:"value"`
	Revision int64  `json:"revision"` // 全局版本号
}

func InitEtcd() {
	if cli != nil {
		return // 如果已经初始化，直接返回
	}
	endpoints := viper.GetStringSlice("etcd.endpoints")
	timeout := viper.GetDuration("etcd.timeout")
	var err error
	cli, err = clientv3.New(clientv3.Config{
		Endpoints:            endpoints,
		DialTimeout:          timeout * time.Second,
		TLS:                  nil, // 如果需要 TLS 支持，可以在这里配置
		Username:             viper.GetString("etcd.auth.username"),
		Password:             viper.GetString("etcd.auth.password"),
		DialKeepAliveTimeout: time.Second * 10, // 设置心跳保持时间
		DialKeepAliveTime:    time.Second * 30, // 设置心跳间隔时间
		MaxCallSendMsgSize:   2 * 1024 * 1024,  // 设置最大发送消息大小为2MB
		MaxCallRecvMsgSize:   2 * 1024 * 1024,  // 设置最大接收消息大小为2MB
		AutoSyncInterval:     time.Second * 5,  // 自动同步间隔时间
	})
	if err != nil {
		panic("failed to connect to etcd: " + err.Error())
	}
	log.Infof("ETCD client initialized with endpoints: %v", endpoints)
	return
}

// Put 设置键值对
func Put(ctx context.Context, key, value string) error {
	log.Infof("🔄 etcd PUT starting: key=%s", key)
	resp, err := cli.Put(ctx, key, value)
	if err != nil {
		log.Errorf("❌ etcd PUT failed: key=%s, error=%v", key, err)
		return err
	}
	log.Infof("✅ etcd PUT success: key=%s, revision=%d", key, resp.Header.Revision)
	return nil
}

// TransactionPut 带事务的设置键值对
func TransactionPut(ctx context.Context, kv map[string]string) error {
	if len(kv) == 0 {
		return nil // 如果没有键值对需要设置，直接返回
	}

	ops := make([]clientv3.Op, 0, len(kv))
	for k, v := range kv {
		ops = append(ops, clientv3.OpPut(k, v))
	}

	txnResp, err := cli.Txn(ctx).Then(ops...).Commit()
	if err != nil {
		return fmt.Errorf("transaction put failed: %w", err)
	}
	if !txnResp.Succeeded {
		return fmt.Errorf("transaction put failed, no keys were updated")
	}
	return nil
}

// Get 获取指定键的值
func Get(ctx context.Context, key string) (Response, error) {
	resp, err := cli.Get(ctx, key)
	if err != nil {
		return Response{}, err
	}
	if len(resp.Kvs) == 0 {
		return Response{}, nil
	}
	if len(resp.Kvs) > 1 {
		return Response{}, fmt.Errorf("unexpected multiple values for key %s", key)
	}
	return Response{
		Key:      resp.Kvs[0].Key,
		Value:    resp.Kvs[0].Value,
		Revision: resp.Header.Revision,
	}, nil
}

// Delete 删除指定键
func Delete(ctx context.Context, key string) error {
	_, err := cli.Delete(ctx, key)
	return err
}

// PutWithLease 带租约的设置键值对
func PutWithLease(ctx context.Context, key, value string, leaseID clientv3.LeaseID) error {
	_, err := cli.Put(ctx, key, value, clientv3.WithLease(leaseID))
	return err
}

// GetWithPrefix 获取指定前缀的所有键值对
func GetWithPrefix(ctx context.Context, prefix string) (*clientv3.GetResponse, error) {
	return cli.Get(ctx, prefix, clientv3.WithPrefix())
}

// Transfer 事件处理函数类型
type Transfer func(result *clientv3.WatchResponse, closed bool)

// Watch 监听指定前缀的变化
func Watch(ctx context.Context, prefix string, revSince int64, transfer Transfer) {
	defer func() {
		transfer(&clientv3.WatchResponse{}, true) // 传递一个空的 WatchResponse 表示关闭
	}()
	rch := cli.Watch(ctx, prefix, clientv3.WithPrefix(), clientv3.WithRev(revSince))
	for {
		select {
		case <-ctx.Done():
			log.Infof("etcd watch stream for prefix %s closed: %v", prefix, ctx.Err())
			return
		case result := <-rch:
			if result.Canceled {
				log.Errorf("watch canceled for prefix %s: %v", prefix, result.Err())
				return
			}
			transfer(&result, false)
		}
	}
}

// Close 关闭客户端连接
func Close() {
	_ = cli.Close()
}
