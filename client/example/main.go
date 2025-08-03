package main

import (
	"context"
	"fmt"
	"time"

	"github.com/yxxchange/pipefree/client"
	"github.com/yxxchange/pipefree/client/informers"
	"github.com/yxxchange/pipefree/client/typed"
	"github.com/yxxchange/pipefree/config"
	"github.com/yxxchange/pipefree/helper/log"
	"github.com/yxxchange/pipefree/infra/dal"
	"github.com/yxxchange/pipefree/infra/dal/model"
	"github.com/yxxchange/pipefree/infra/etcd"
)

func main() {
	// 初始化配置
	config.Init("../../config.yaml")
	dal.InitDB()
	etcd.InitEtcd()

	// 创建客户端
	clientset := client.NewForConfig()

	// 示例1: 直接使用List和Watch
	fmt.Println("=== 示例1: 直接使用List和Watch ===")
	directExample(clientset)

	// 示例2: 使用Informer模式
	fmt.Println("\n=== 示例2: 使用Informer模式 ===")
	informerExample(clientset)
}

// directExample 直接使用客户端示例
func directExample(clientset client.Interface) {
	ctx := context.Background()

	// List操作
	fmt.Println("执行List操作...")
	listOpts := typed.ListOptions{
		Namespace: "default",
		Kind:      "task",
	}
	
	list, err := clientset.NodeExecs().List(ctx, listOpts)
	if err != nil {
		log.Errorf("List failed: %v", err)
		return
	}
	
	fmt.Printf("找到 %d 个节点执行记录\n", len(list.Items))
	for _, item := range list.Items {
		fmt.Printf("- %s/%s (ID: %d, Phase: %s)\n", 
			item.Namespace, item.Name, item.Id, item.Phase.Phase)
	}

	// Watch操作
	fmt.Println("\n开始Watch操作...")
	watchOpts := typed.WatchOptions{
		Namespace: "default",
		Kind:      "task",
	}
	
	watcher, err := clientset.NodeExecs().Watch(ctx, watchOpts)
	if err != nil {
		log.Errorf("Watch failed: %v", err)
		return
	}
	defer watcher.Stop()

	// 监听5秒钟
	timeout := time.After(5 * time.Second)
	for {
		select {
		case event, ok := <-watcher.ResultChan():
			if !ok {
				fmt.Println("Watch channel closed")
				return
			}
			
			if event.Error != nil {
				log.Errorf("Watch error: %v", event.Error)
				continue
			}
			
			fmt.Printf("收到事件: %s - %s/%s (ID: %d)\n", 
				event.Type, event.Object.Namespace, event.Object.Name, event.Object.Id)
				
		case <-timeout:
			fmt.Println("Watch超时，结束监听")
			return
		}
	}
}

// informerExample 使用Informer模式示例
func informerExample(clientset client.Interface) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 创建Informer工厂
	factory := informers.NewSharedInformerFactory(clientset.NodeExecs(), 30*time.Second)

	// 获取NodeExec Informer
	nodeExecInformer := factory.NodeExecs().V1().NodeExecs()

	// 添加事件处理器
	nodeExecInformer.Informer().AddEventHandler(informers.ResourceEventHandlerFuncs{
		AddFunc: func(obj *model.NodeExec) {
			fmt.Printf("[Informer] 添加: %s/%s (ID: %d, Phase: %s)\n", 
				obj.Namespace, obj.Name, obj.Id, obj.Phase.Phase)
		},
		UpdateFunc: func(oldObj, newObj *model.NodeExec) {
			fmt.Printf("[Informer] 更新: %s/%s (ID: %d, Phase: %s -> %s)\n", 
				newObj.Namespace, newObj.Name, newObj.Id, 
				oldObj.Phase.Phase, newObj.Phase.Phase)
		},
		DeleteFunc: func(obj *model.NodeExec) {
			fmt.Printf("[Informer] 删除: %s/%s (ID: %d)\n", 
				obj.Namespace, obj.Name, obj.Id)
		},
	})

	// 启动Informer
	factory.Start(ctx)

	// 等待缓存同步
	fmt.Println("等待缓存同步...")
	if !factory.WaitForCacheSync(ctx) {
		fmt.Println("缓存同步失败")
		return
	}
	fmt.Println("缓存同步完成")

	// 使用Lister查询本地缓存
	lister := nodeExecInformer.Lister()
	
	// 列出所有对象
	allObjects, err := lister.List("", "")
	if err != nil {
		log.Errorf("Lister.List failed: %v", err)
		return
	}
	fmt.Printf("本地缓存中有 %d 个对象\n", len(allObjects))

	// 按命名空间和类型查询
	defaultTasks, err := lister.List("default", "task")
	if err != nil {
		log.Errorf("Lister.List failed: %v", err)
		return
	}
	fmt.Printf("default命名空间中的task类型对象: %d 个\n", len(defaultTasks))

	// 运行10秒钟观察事件
	fmt.Println("运行10秒钟观察事件...")
	time.Sleep(10 * time.Second)
	
	fmt.Println("示例结束")
}