package pipe_exec

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/yxxchange/pipefree/http/common"
	"github.com/yxxchange/pipefree/service/pipe_exec"
)

const (
	routeGroup = "/pipe_exec"
)

func RegisterV1(router *gin.RouterGroup) {
	group := router.Group(routeGroup)
	{
		group.POST("/:pipe_id", Run)
		group.GET("/list", List)
		group.GET("/get", Get)
	}
}

func Run(c *gin.Context) {
	var req PipeExecReqParam
	if err := c.ShouldBindUri(&req); err != nil {
		common.ResponseError(c, -1, "invalid request parameters")
		return
	}
	if req.PipeId <= 0 {
		common.ResponseError(c, -1, "pipe id is required")
		return
	}
	err := pipe_exec.NewService(c).Run(req.PipeId)
	if err != nil {
		common.ResponseError(c, pipe_exec.ErrorCode, err.Error())
		return
	}
	common.ResponseOk(c, "pipe execution started successfully")
}

func List(c *gin.Context) {
	namespace := c.Query("namespace")
	kind := c.Query("kind")
	limitStr := c.Query("limit")
	
	var limit int64
	if limitStr != "" {
		if l, err := strconv.ParseInt(limitStr, 10, 64); err == nil {
			limit = l
		}
	}
	
	service := pipe_exec.NewService(c)
	list, err := service.List(namespace, kind, limit)
	if err != nil {
		common.ResponseError(c, pipe_exec.ErrorCode, err.Error())
		return
	}
	
	common.ResponseOk(c, list)
}

func Get(c *gin.Context) {
	namespace := c.Query("namespace")
	kind := c.Query("kind")
	idStr := c.Query("id")
	
	if namespace == "" || kind == "" || idStr == "" {
		common.ResponseError(c, -1, "namespace, kind and id are required")
		return
	}
	
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		common.ResponseError(c, -1, "invalid id parameter")
		return
	}
	
	service := pipe_exec.NewService(c)
	nodeExec, err := service.Get(namespace, kind, id)
	if err != nil {
		common.ResponseError(c, pipe_exec.ErrorCode, err.Error())
		return
	}
	
	common.ResponseOk(c, nodeExec)
}
