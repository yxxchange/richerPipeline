package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/yxxchange/pipefree/client_pipe/typed"
	"github.com/yxxchange/pipefree/config"
	"github.com/yxxchange/pipefree/infra"
	"github.com/yxxchange/pipefree/infra/dal/model"
	"github.com/yxxchange/pipefree/service/pipe_exec"
)

func main() {
	// 初始化配置和基础设施
	config.InitConfig()
	infra.Init()

	ctx := context.Background()

	// 创建客户端
	client := typed.NewNodeExecClient()

	// 启动 Watch
	fmt.Println("🔍 Starting watch...")
	watcher, err := client.Watch(ctx, typed.WatchOptions{
		Namespace: "default",
		Kind:      "task",
	})
	if err != nil {
		log.Fatalf("Failed to start watch: %v", err)
	}
	defer watcher.Stop()

	// 在另一个 goroutine 中监听事件
	go func() {
		for {
			select {
			case event, ok := <-watcher.ResultChan():
				if !ok {
					fmt.Println("❌ Watch channel closed")
					return
				}
				
				if event.Error != nil {
					fmt.Printf("❌ Watch error: %v\n", event.Error)
					continue
				}
				
				fmt.Printf("📨 Received event: Type=%s\n", event.Type)
				if event.Object != nil {
					fmt.Printf("   Object: ID=%d, Name=%s, Phase=%s\n", 
						event.Object.Id, event.Object.Name, event.Object.Phase.Phase)
				}
				
			case <-ctx.Done():
				return
			}
		}
	}()

	// 等待一下让 watch 建立连接
	time.Sleep(2 * time.Second)

	// 创建测试服务
	service := pipe_exec.NewService(ctx)

	// 模拟更新节点状态
	fmt.Println("🔄 Updating node status...")
	
	testNode := &model.NodeExec{
		Id:        1,
		Name:      "test-node",
		Namespace: "default",
		Kind:      "task",
		Phase: model.NodePhase{
			Phase: model.NodePhaseRunning,
		},
		PipeExecId: 1,
		PipeCfgId:  1,
	}

	// 执行更新操作
	updatedNode, err := service.Update(testNode)
	if err != nil {
		log.Printf("❌ Failed to update node: %v", err)
	} else {
		fmt.Printf("✅ Node updated successfully: %s\n", updatedNode.Phase.Phase)
	}

	// 再次更新状态
	time.Sleep(1 * time.Second)
	testNode.Phase.Phase = model.NodePhaseSucceeded
	
	fmt.Println("🔄 Updating node to succeeded...")
	updatedNode, err = service.Update(testNode)
	if err != nil {
		log.Printf("❌ Failed to update node: %v", err)
	} else {
		fmt.Printf("✅ Node updated to succeeded: %s\n", updatedNode.Phase.Phase)
	}

	// 等待事件处理
	fmt.Println("⏳ Waiting for events...")
	time.Sleep(5 * time.Second)
	
	fmt.Println("🏁 Test completed")
}