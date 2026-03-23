package pipeline

import (
	"fmt"
	"strings"
	"time"
)

var taskRetryDelays = []time.Duration{
	time.Minute,
	3 * time.Minute,
	10 * time.Minute,
}

func TaskRetryDelay(attempt int) time.Duration {
	if attempt <= 0 {
		return taskRetryDelays[0]
	}
	index := attempt - 1
	if index >= len(taskRetryDelays) {
		index = len(taskRetryDelays) - 1
	}
	return taskRetryDelays[index]
}

func TaskRetryQueueName(base string, attempt int) string {
	return fmt.Sprintf("%s.%s", strings.TrimSpace(base), retryDelaySuffix(TaskRetryDelay(attempt)))
}

func TaskRetryRoutingKey(base string, attempt int) string {
	return fmt.Sprintf("%s.%s", strings.TrimSpace(base), retryDelaySuffix(TaskRetryDelay(attempt)))
}

func retryDelaySuffix(delay time.Duration) string {
	if delay%time.Minute == 0 {
		return fmt.Sprintf("%dm", int(delay/time.Minute))
	}
	return fmt.Sprintf("%ds", int(delay/time.Second))
}
