package client

import (
	"fmt"
	"net/http"
	"time"

	"api/pkg/database"
	"api/pkg/deepseek"
	"api/pkg/queue"
	pkgredis "api/pkg/redis"
	"api/pkg/response"

	"github.com/gin-gonic/gin"
)

type SystemController struct {
	BaseAPIController
}

// Health 用于容器联通性检查
func (ctrl *SystemController) Health(c *gin.Context) {
	dbStatus := "up"
	redisStatus := "up"
	rabbitStatus := "up"

	if database.SQLDB == nil || database.SQLDB.Ping() != nil {
		dbStatus = "down"
	}
	if pkgredis.Redis == nil || pkgredis.Redis.Ping() != nil {
		redisStatus = "down"
	}
	if !queue.Enabled() {
		rabbitStatus = "disabled"
	} else if queue.Ping() != nil {
		rabbitStatus = "down"
	}

	status := "ok"
	if dbStatus != "up" || redisStatus != "up" || (queue.Enabled() && rabbitStatus != "up") {
		status = "degraded"
	}

	response.JSON(c, gin.H{
		"status": status,
		"db":     dbStatus,
		"redis":  redisStatus,
		"rabbit": rabbitStatus,
		"time":   time.Now().Format(time.RFC3339),
	})
}

// PushTask 推送任务到 RabbitMQ
func (ctrl *SystemController) PushTask(c *gin.Context) {
	if !queue.Enabled() {
		c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
			"message": "rabbitmq is disabled on this environment",
		})
		return
	}

	task := c.Query("task")
	if task == "" {
		task = fmt.Sprintf("test_task_%d", time.Now().Unix())
	}
	taskType := c.DefaultQuery("type", "video.generate")

	taskID, err := queue.PublishVideoTask(taskType, map[string]interface{}{
		"task": task,
	})
	if err != nil {
		response.Abort500(c, "push task failed: "+err.Error())
		return
	}

	response.JSON(c, gin.H{
		"message":   "task pushed",
		"task_id":   taskID,
		"task":      task,
		"task_type": taskType,
		"queue":     "rabbitmq",
	})
}

// DeepSeekTest backend 直接调用 DeepSeek，立即返回成功或失败。
func (ctrl *SystemController) DeepSeekTest(c *gin.Context) {
	prompt := c.Query("prompt")
	if prompt == "" {
		prompt = `Return strict JSON only: {"ok":true,"provider":"deepseek","msg":"ping"}`
	}
	testID := fmt.Sprintf("deepseek-test-%d", time.Now().UnixNano())

	content, err := deepseek.RunDeepSeekTest(deepseek.LoadConfig(), prompt)
	if err != nil {
		c.AbortWithStatusJSON(502, gin.H{
			"message":   "deepseek test failed",
			"test_id":   testID,
			"task_type": "deepseek.test",
			"error":     err.Error(),
		})
		return
	}

	response.JSON(c, gin.H{
		"message":   "deepseek test ok",
		"test_id":   testID,
		"task_type": "deepseek.test",
		"content":   content,
	})
}
