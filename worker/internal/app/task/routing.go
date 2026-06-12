package task

import (
	"fmt"
	"strings"

	conf "worker/pkg/config"
)

const (
	WorkerRoleAll   = "all"
	WorkerRoleMain  = "main"
	WorkerRoleAlign = "align"
)

type QueueRoute struct {
	Queue               string
	RoutingKey          string
	RetryQueueBase      string
	RetryRoutingKeyBase string
	DLQ                 string
	DLQRoutingKey       string
}

func SplitQueuesEnabled() bool {
	return conf.Get[bool]("worker.rabbitmq_split_queues")
}

func NormalizeWorkerRole(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case WorkerRoleMain:
		return WorkerRoleMain
	case WorkerRoleAlign:
		return WorkerRoleAlign
	default:
		return WorkerRoleAll
	}
}

func QueueForWorkerRole(role string) (string, error) {
	if !SplitQueuesEnabled() {
		return mainQueueRoute().Queue, nil
	}

	switch NormalizeWorkerRole(role) {
	case WorkerRoleMain:
		return mainQueueRoute().Queue, nil
	case WorkerRoleAlign:
		return alignQueueRoute().Queue, nil
	default:
		return "", fmt.Errorf("worker_role must be main or align when RABBITMQ_SPLIT_QUEUES=true")
	}
}

func RabbitMQRoutes() []QueueRoute {
	routes := []QueueRoute{mainQueueRoute()}
	if SplitQueuesEnabled() {
		routes = append(routes, alignQueueRoute())
	}
	return routes
}

func RouteForTaskType(taskType string) QueueRoute {
	if SplitQueuesEnabled() && IsAlignTaskType(taskType) {
		return alignQueueRoute()
	}
	return mainQueueRoute()
}

func IsAlignTaskType(taskType string) bool {
	switch strings.TrimSpace(taskType) {
	case "podcast.audio.align.v1", "practical.audio.align.v1":
		return true
	default:
		return false
	}
}

func mainQueueRoute() QueueRoute {
	return QueueRoute{
		Queue:               conf.Get[string]("worker.rabbitmq_queue"),
		RoutingKey:          conf.Get[string]("worker.rabbitmq_routing_key"),
		RetryQueueBase:      conf.Get[string]("worker.rabbitmq_retry_queue"),
		RetryRoutingKeyBase: conf.Get[string]("worker.rabbitmq_retry_routing_key"),
		DLQ:                 conf.Get[string]("worker.rabbitmq_dlq"),
		DLQRoutingKey:       conf.Get[string]("worker.rabbitmq_dlq_routing_key"),
	}
}

func alignQueueRoute() QueueRoute {
	return QueueRoute{
		Queue:               conf.Get[string]("worker.rabbitmq_align_queue"),
		RoutingKey:          conf.Get[string]("worker.rabbitmq_align_routing_key"),
		RetryQueueBase:      conf.Get[string]("worker.rabbitmq_align_retry_queue"),
		RetryRoutingKeyBase: conf.Get[string]("worker.rabbitmq_align_retry_routing_key"),
		DLQ:                 conf.Get[string]("worker.rabbitmq_align_dlq"),
		DLQRoutingKey:       conf.Get[string]("worker.rabbitmq_align_dlq_routing_key"),
	}
}
