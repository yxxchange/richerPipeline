package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/yxxchange/pipefree/client_pipe"
	"github.com/yxxchange/pipefree/client_pipe/informers/nodeexec"
	"github.com/yxxchange/pipefree/client_pipe/typed"
	"github.com/yxxchange/pipefree/infra/dal/model"
)

func main() {
	// 创建客户端
	clientset := client_pipe.NewClientSet()

	// 示例1: 直接使用 typed client 进行 CRUD 操作
	fmt.Println("=== Typed Client Example ===")
	if err := typedClientExample(clientset); err != nil {
		log.Fatalf("Typed client example failed: %v", err)
	}

	// 示例2: 使用 Informer 监听资源变化
	fmt.Println("\n=== Informer Example ===")
	if err := informerExample(clientset); err != nil {
		log.Fatalf("Informer example failed: %v", err)
	}
}

// typedClientExample 展示如何使用 typed client
func typedClientExample(clientset client_pipe.Interface) error {
	ctx := context.Background()
	nodeExecClient := clientset.NodeExecs()

	// List 操作
	fmt.Println("Listing NodeExecs...")
	list, err := nodeExecClient.List(ctx, typed.ListOptions{
		Namespace: "default",
		Kind:      "pipeline",
	})
	if err != nil {
		return fmt.Errorf("failed to list: %w", err)
	}
	fmt.Printf("Found %d NodeExecs\n", len(list.Items))

	// Get 操作
	if len(list.Items) > 0 {
		first := list.Items[0]
		fmt.Printf("Getting NodeExec: %s/%s/%d\n", first.Namespace, first.Kind, first.Id)

		obj, err := nodeExecClient.Get(ctx, first.Namespace, first.Kind, first.Id)
		if err != nil {
			return fmt.Errorf("failed to get: %w", err)
		}
		fmt.Printf("Got NodeExec: %s (Phase: %s)\n", obj.Name, obj.Phase.Phase)
	}

	return nil
}

// informerExample 展示如何使用 Informer
func informerExample(clientset client_pipe.Interface) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 获取 Informer Factory
	factory := clientset.Informers()

	// 获取 NodeExec Informer
	nodeExecInformer := factory.NodeExecs().V1().NodeExecs("default", "pipeline")

	// 添加事件处理器
	nodeExecInformer.AddEventHandler(nodeexec.ResourceEventHandlerFuncs{
		AddFunc: func(obj *model.NodeExec) {
			fmt.Printf("NodeExec ADDED: %s/%s/%d (Phase: %s)\n",
				obj.Namespace, obj.Kind, obj.Id, obj.Phase.Phase)
		},
		UpdateFunc: func(oldObj, newObj *model.NodeExec) {
			fmt.Printf("NodeExec UPDATED: %s/%s/%d (Phase: %s -> %s)\n",
				newObj.Namespace, newObj.Kind, newObj.Id, oldObj.Phase.Phase, newObj.Phase.Phase)
		},
		DeleteFunc: func(obj *model.NodeExec) {
			fmt.Printf("NodeExec DELETED: %s/%s/%d\n",
				obj.Namespace, obj.Kind, obj.Id)
		},
	})

	// 启动 Informer Factory
	factory.Start(ctx)

	// 等待缓存同步
	fmt.Println("Waiting for cache sync...")
	if !factory.WaitForCacheSync(ctx) {
		return fmt.Errorf("failed to sync cache")
	}
	fmt.Println("Cache synced!")

	// 使用 Lister 查询本地缓存
	lister := factory.NodeExecs().V1().NodeExecs("default", "pipeline").Lister()

	// 从缓存中列出对象
	cachedList, err := lister.List("default", "pipeline")
	if err != nil {
		return fmt.Errorf("failed to list from cache: %w", err)
	}
	fmt.Printf("Found %d NodeExecs in cache\n", len(cachedList))

	// 从缓存中获取特定对象
	if len(cachedList) > 0 {
		first := cachedList[0]
		cachedObj, err := lister.Get("default", "pipeline", first.Id)
		if err != nil {
			return fmt.Errorf("failed to get from cache: %w", err)
		}
		fmt.Printf("Got from cache: %s (Phase: %s)\n", cachedObj.Name, cachedObj.Phase.Phase)
	}

	// 保持运行以观察事件
	fmt.Println("Watching for events... (will run for 25 seconds)")
	time.Sleep(25 * time.Second)

	return nil
}

// 高级用法示例
func advancedExample() {
	// 自定义配置
	config := client_pipe.ClientConfig{
		DefaultResyncPeriod: 10 * time.Second, // 更频繁的重新同步
	}

	clientset := client_pipe.NewClientSetWithConfig(config)
	factory := clientset.Informers()

	// 监听多个 namespace 和 kind
	namespaces := []string{"default", "system", "user"}
	kinds := []string{"pipeline", "task", "job"}

	ctx := context.Background()

	for _, ns := range namespaces {
		for _, kind := range kinds {
			informer := factory.GetInformer(ns, kind)
			informer.AddEventHandler(nodeexec.ResourceEventHandlerFuncs{
				AddFunc: func(obj *model.NodeExec) {
					fmt.Printf("[%s/%s] Added: %s\n", obj.Namespace, obj.Kind, obj.Name)
				},
				UpdateFunc: func(oldObj, newObj *model.NodeExec) {
					if oldObj.Phase.Phase != newObj.Phase.Phase {
						fmt.Printf("[%s/%s] Phase changed: %s -> %s\n",
							newObj.Namespace, newObj.Kind, oldObj.Phase.Phase, newObj.Phase.Phase)
					}
				},
				DeleteFunc: func(obj *model.NodeExec) {
					fmt.Printf("[%s/%s] Deleted: %s\n", obj.Namespace, obj.Kind, obj.Name)
				},
			})
		}
	}

	// 启动所有 Informer
	factory.Start(ctx)
	factory.WaitForCacheSync(ctx)

	fmt.Println("All informers started and synced!")
}
