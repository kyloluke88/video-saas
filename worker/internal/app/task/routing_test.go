package task

import (
	"testing"

	_ "worker/config"
	conf "worker/pkg/config"
)

func configureTaskRouting(t *testing.T, splitQueues bool) {
	t.Helper()
	t.Setenv("RABBITMQ_SPLIT_QUEUES", boolEnv(splitQueues))
	t.Setenv("RABBITMQ_QUEUE", "video.tasks.generate")
	t.Setenv("RABBITMQ_ROUTING_KEY", "video.generate")
	t.Setenv("RABBITMQ_RETRY_QUEUE", "video.tasks.generate.retry")
	t.Setenv("RABBITMQ_RETRY_ROUTING_KEY", "video.generate.retry")
	t.Setenv("RABBITMQ_DLQ", "video.tasks.generate.dlq")
	t.Setenv("RABBITMQ_DLQ_ROUTING_KEY", "video.generate.dlq")
	t.Setenv("RABBITMQ_ALIGN_QUEUE", "video.tasks.align")
	t.Setenv("RABBITMQ_ALIGN_ROUTING_KEY", "video.align")
	t.Setenv("RABBITMQ_ALIGN_RETRY_QUEUE", "video.tasks.align.retry")
	t.Setenv("RABBITMQ_ALIGN_RETRY_ROUTING_KEY", "video.align.retry")
	t.Setenv("RABBITMQ_ALIGN_DLQ", "video.tasks.align.dlq")
	t.Setenv("RABBITMQ_ALIGN_DLQ_ROUTING_KEY", "video.align.dlq")
	conf.InitConfig("")
}

func boolEnv(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

func TestQueueForWorkerRoleWhenSplitQueuesEnabled(t *testing.T) {
	configureTaskRouting(t, true)

	mainQueue, err := QueueForWorkerRole(WorkerRoleMain)
	if err != nil {
		t.Fatalf("main queue error: %v", err)
	}
	if mainQueue != "video.tasks.generate" {
		t.Fatalf("unexpected main queue: %s", mainQueue)
	}

	alignQueue, err := QueueForWorkerRole(WorkerRoleAlign)
	if err != nil {
		t.Fatalf("align queue error: %v", err)
	}
	if alignQueue != "video.tasks.align" {
		t.Fatalf("unexpected align queue: %s", alignQueue)
	}

	if _, err := QueueForWorkerRole(WorkerRoleAll); err == nil {
		t.Fatal("expected role validation error for worker_role=all when split queues are enabled")
	}
}

func TestRouteForTaskTypeRoutesAlignTasksToAlignQueue(t *testing.T) {
	configureTaskRouting(t, true)

	alignRoute := RouteForTaskType("podcast.audio.align.v1")
	if alignRoute.Queue != "video.tasks.align" || alignRoute.RoutingKey != "video.align" {
		t.Fatalf("unexpected align route: %#v", alignRoute)
	}

	mainRoute := RouteForTaskType("podcast.compose.render.v1")
	if mainRoute.Queue != "video.tasks.generate" || mainRoute.RoutingKey != "video.generate" {
		t.Fatalf("unexpected main route: %#v", mainRoute)
	}
}

func TestRouteForTaskTypeFallsBackToMainQueueWhenSplitQueuesDisabled(t *testing.T) {
	configureTaskRouting(t, false)

	route := RouteForTaskType("podcast.audio.align.v1")
	if route.Queue != "video.tasks.generate" || route.RoutingKey != "video.generate" {
		t.Fatalf("unexpected fallback route: %#v", route)
	}
}
