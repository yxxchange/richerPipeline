package server

import (
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"github.com/yxxchange/pipefree/http/route"
)

func LaunchServer() error {
	// Initialize Gin router
	server := gin.Default()
	route.RegisterV1Routes(server)
	// Start the server
	return server.Run(":" + viper.GetString("http.port"))
}

func NewServer() *gin.Engine {
	// Initialize Gin router
	server := gin.Default()
	route.RegisterV1Routes(server)
	return server
}
