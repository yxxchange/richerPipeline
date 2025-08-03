package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/yxxchange/pipefree/config"
	"github.com/yxxchange/pipefree/http/server"
	"github.com/yxxchange/pipefree/infra/dal"
	"github.com/yxxchange/pipefree/infra/etcd"
)

func main() {
	config.Init("./config.yaml")
	dal.InitDB()
	etcd.InitEtcd()

	// 启动 HTTP 服务器
	srv := server.NewServer()
	httpServer := &http.Server{
		Addr:    ":8080",
		Handler: srv,
	}

	// 启动服务器
	go func() {
		log.Println("Server starting on :8080")
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	// 优雅关闭
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown:", err)
	}

	log.Println("Server exited")
}
