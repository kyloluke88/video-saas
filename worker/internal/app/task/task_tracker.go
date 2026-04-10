package task

import (
	"context"
	"errors"
	"strings"
	"time"

	"worker/internal/persistence"
	"worker/pkg/x/mapx"
)

type TaskTracker interface {
	OnTaskStart(task VideoTaskMessage, retries int) error
	OnTaskRetry(task VideoTaskMessage, retries int, taskErr error) error
	OnTaskFailed(task VideoTaskMessage, retries int, taskErr error) error
	OnTaskCancelled(task VideoTaskMessage, retries int) error
	OnTaskSucceeded(task VideoTaskMessage, retries int) error
}

type NoopTaskTracker struct{}

func (NoopTaskTracker) OnTaskStart(VideoTaskMessage, int) error         { return nil }
func (NoopTaskTracker) OnTaskRetry(VideoTaskMessage, int, error) error  { return nil }
func (NoopTaskTracker) OnTaskFailed(VideoTaskMessage, int, error) error { return nil }
func (NoopTaskTracker) OnTaskCancelled(VideoTaskMessage, int) error     { return nil }
func (NoopTaskTracker) OnTaskSucceeded(VideoTaskMessage, int) error     { return nil }

type ProjectTaskTracker struct{}

func NewTaskTracker() TaskTracker {
	return ProjectTaskTracker{}
}

func (ProjectTaskTracker) OnTaskStart(task VideoTaskMessage, retries int) error {
	store, err := persistence.DefaultStore()
	if err != nil {
		return err
	}

	now := time.Now().UTC()
	contentType := taskContentType(task)
	projectID := taskProjectID(task)
	if projectID == "" || contentType == "" {
		return nil
	}
	cancelled, err := projectCancelled(store, projectID)
	if err != nil {
		return err
	}
	if cancelled {
		return CancelledError{ProjectID: projectID}
	}

	retryNum := retries
	return store.UpdateProject(persistence.ProjectUpdate{
		ProjectID:       projectID,
		ContentType:     contentType,
		RunMode:         taskRunMode(task),
		RetryNum:        &retryNum,
		Status:          persistence.ProjectStatusRunning,
		CurrentStage:    trackedTaskStage(task),
		CurrentTaskType: task.TaskType,
		LastError:       "",
		StartedAt:       &now,
	})
}

func (ProjectTaskTracker) OnTaskRetry(task VideoTaskMessage, retries int, taskErr error) error {
	return updateTrackedTask(task, retries+1, persistence.ProjectStatusRetrying, taskErr, nil)
}

func (ProjectTaskTracker) OnTaskFailed(task VideoTaskMessage, retries int, taskErr error) error {
	finishedAt := time.Now().UTC()
	return updateTrackedTask(task, retries, persistence.ProjectStatusError, taskErr, &finishedAt)
}

func (ProjectTaskTracker) OnTaskCancelled(task VideoTaskMessage, retries int) error {
	finishedAt := time.Now().UTC()
	return updateTrackedTask(task, retries, persistence.ProjectStatusCancelled, CancelledError{ProjectID: taskProjectID(task)}, &finishedAt)
}

func (ProjectTaskTracker) OnTaskSucceeded(task VideoTaskMessage, retries int) error {
	finishedAt := time.Now().UTC()
	if isTerminalProjectTask(task) {
		return updateTrackedTask(task, retries, persistence.ProjectStatusFinished, nil, &finishedAt)
	}
	return updateTrackedTask(task, retries, persistence.ProjectStatusRunning, nil, nil)
}

func updateTrackedTask(task VideoTaskMessage, retryNum int, status int16, taskErr error, finishedAt *time.Time) error {
	store, err := persistence.DefaultStore()
	if err != nil {
		return err
	}

	contentType := taskContentType(task)
	projectID := taskProjectID(task)
	if projectID == "" || contentType == "" {
		return nil
	}

	project, err := store.FindProjectByProjectID(projectID)
	if err != nil {
		return err
	}
	if status == persistence.ProjectStatusCancelled {
		if persistence.IsCancelledProjectStatus(project.Status) {
			return nil
		}
	} else if persistence.IsCancellationRequestedStatus(project.Status) {
		return nil
	}

	lastError := ""
	if taskErr != nil && !errors.Is(taskErr, context.Canceled) {
		lastError = taskErr.Error()
	}

	terminatedTaskType := ""
	if status == persistence.ProjectStatusError || status == persistence.ProjectStatusCancelled {
		terminatedTaskType = task.TaskType
	}

	cancelledAt := (*time.Time)(nil)
	if status == persistence.ProjectStatusCancelled {
		cancelledAt = finishedAt
	}

	return store.UpdateProject(persistence.ProjectUpdate{
		ProjectID:          projectID,
		ContentType:        contentType,
		RunMode:            taskRunMode(task),
		RetryNum:           &retryNum,
		Status:             status,
		CurrentStage:       trackedTaskStage(task),
		CurrentTaskType:    task.TaskType,
		TerminatedTaskType: terminatedTaskType,
		LastError:          lastError,
		FinishedAt:         finishedAt,
		CancelledAt:        cancelledAt,
	})
}

func trackedTaskStage(task VideoTaskMessage) string {
	if task.TaskType == "podcast.audio.generate.v1" {
		if runMode := taskRunMode(task); runMode != nil && *runMode == 2 {
			return "compose"
		}
	}
	return taskStage(task.TaskType)
}

func taskContentType(task VideoTaskMessage) string {
	if task.Payload != nil {
		if raw := strings.TrimSpace(mapx.GetString(task.Payload, "content_type")); raw != "" {
			return raw
		}
	}
	switch {
	case strings.HasPrefix(task.TaskType, "podcast."):
		return "podcast"
	case task.TaskType == "upload.v1" && strings.Contains(strings.ToLower(taskProjectID(task)), "podcast"):
		return "podcast"
	case strings.HasPrefix(task.TaskType, "scene.") || task.TaskType == "plan.v1" || task.TaskType == "compose.v1":
		return "idiom"
	default:
		return ""
	}
}

func taskStage(taskType string) string {
	switch taskType {
	case "podcast.audio.generate.v1":
		return "audio_generate"
	case "podcast.audio.align.v1":
		return "audio_align"
	case "podcast.compose.render.v1":
		return "compose"
	case "podcast.compose.finalize.v1":
		return "compose"
	case "upload.v1":
		return "upload"
	case "podcast.page.persist.v1":
		return "script_persist"
	case "plan.v1":
		return "plan"
	case "scene.generate.v1":
		return "scene_generate"
	case "compose.v1":
		return "compose"
	default:
		return "unknown"
	}
}

func taskRunMode(task VideoTaskMessage) *int {
	if task.Payload == nil {
		return nil
	}
	value := mapx.GetInt(task.Payload, "run_mode", 0)
	runMode := value
	return &runMode
}

func isTerminalProjectTask(task VideoTaskMessage) bool {
	switch taskContentType(task) {
	case "podcast":
		return task.TaskType == "upload.v1"
	case "idiom":
		return task.TaskType == "upload.v1"
	default:
		return false
	}
}

func projectCancelled(store *persistence.Store, projectID string) (bool, error) {
	if store == nil || projectID == "" {
		return false, nil
	}

	project, err := store.FindProjectByProjectID(projectID)
	if err != nil {
		return false, err
	}
	return persistence.IsCancellationRequestedStatus(project.Status), nil
}

func MarkPodcastProjectPersisted(projectID string) error {
	store, err := persistence.DefaultStore()
	if err != nil {
		return err
	}
	cancelled, err := projectCancelled(store, projectID)
	if err != nil {
		return err
	}
	if cancelled {
		return nil
	}

	retryNum := 0
	return store.UpdateProject(persistence.ProjectUpdate{
		ProjectID:       projectID,
		ContentType:     "podcast",
		RetryNum:        &retryNum,
		Status:          persistence.ProjectStatusRunning,
		CurrentStage:    "script_persist",
		CurrentTaskType: "podcast.page.persist.v1",
		LastError:       "",
	})
}

func UpdatePodcastProjectUpload(projectID string) error {
	store, err := persistence.DefaultStore()
	if err != nil {
		return err
	}
	cancelled, err := projectCancelled(store, projectID)
	if err != nil {
		return err
	}
	if cancelled {
		return nil
	}

	retryNum := 0
	return store.UpdateProject(persistence.ProjectUpdate{
		ProjectID:       projectID,
		ContentType:     "podcast",
		RetryNum:        &retryNum,
		Status:          persistence.ProjectStatusRunning,
		CurrentStage:    "upload",
		CurrentTaskType: "upload.v1",
	})
}

func FinalizePodcastProjectUpload(projectID string) error {
	store, err := persistence.DefaultStore()
	if err != nil {
		return err
	}
	cancelled, err := projectCancelled(store, projectID)
	if err != nil {
		return err
	}
	if cancelled {
		return nil
	}

	now := time.Now().UTC()
	retryNum := 0
	return store.UpdateProject(persistence.ProjectUpdate{
		ProjectID:       projectID,
		ContentType:     "podcast",
		RetryNum:        &retryNum,
		Status:          persistence.ProjectStatusFinished,
		CurrentStage:    "upload",
		CurrentTaskType: "upload.v1",
		LastError:       "",
		FinishedAt:      &now,
	})
}
