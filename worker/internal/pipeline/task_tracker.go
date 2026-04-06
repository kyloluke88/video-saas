package pipeline

import (
	"encoding/json"
	"strings"
	"time"

	"worker/internal/dto"
	"worker/internal/persistence"
	"worker/pkg/helpers"
)

type TaskTracker interface {
	OnTaskStart(task dto.VideoTaskMessage, retries int) error
	OnTaskRetry(task dto.VideoTaskMessage, retries int, taskErr error) error
	OnTaskFailed(task dto.VideoTaskMessage, retries int, taskErr error) error
	OnTaskSucceeded(task dto.VideoTaskMessage, retries int) error
}

type NoopTaskTracker struct{}

func (NoopTaskTracker) OnTaskStart(dto.VideoTaskMessage, int) error         { return nil }
func (NoopTaskTracker) OnTaskRetry(dto.VideoTaskMessage, int, error) error  { return nil }
func (NoopTaskTracker) OnTaskFailed(dto.VideoTaskMessage, int, error) error { return nil }
func (NoopTaskTracker) OnTaskSucceeded(dto.VideoTaskMessage, int) error     { return nil }

type ProjectTaskTracker struct{}

func NewTaskTracker() TaskTracker {
	return ProjectTaskTracker{}
}

func (ProjectTaskTracker) OnTaskStart(task dto.VideoTaskMessage, retries int) error {
	store, err := persistence.DefaultStore()
	if err != nil {
		return err
	}

	now := time.Now().UTC()
	payloadJSON, err := json.Marshal(task.Payload)
	if err != nil {
		return err
	}

	contentType := taskContentType(task)
	projectID := taskProjectID(task)
	if projectID == "" || contentType == "" {
		return nil
	}

	if err := store.UpsertVideoProject(persistence.VideoProjectUpsert{
		ProjectID:       projectID,
		ContentType:     contentType,
		Language:        taskLanguage(task),
		Title:           taskTitle(task),
		Status:          "running",
		CurrentStage:    taskStage(task.TaskType),
		CurrentTaskType: task.TaskType,
		LastError:       "",
		RequestPayload:  rootRequestPayload(task, payloadJSON),
		StartedAt:       &now,
	}); err != nil {
		return err
	}

	return store.UpsertTaskRun(persistence.TaskRunUpsert{
		ProjectID:  projectID,
		TaskID:     task.TaskID,
		TaskType:   task.TaskType,
		Stage:      taskStage(task.TaskType),
		Status:     "running",
		RetryCount: retries,
		Payload:    payloadJSON,
		StartedAt:  now,
	})
}

func (ProjectTaskTracker) OnTaskRetry(task dto.VideoTaskMessage, retries int, taskErr error) error {
	finishedAt := time.Now().UTC()
	return updateTrackedTask(task, retries, "retrying", taskErr, &finishedAt)
}

func (ProjectTaskTracker) OnTaskFailed(task dto.VideoTaskMessage, retries int, taskErr error) error {
	finishedAt := time.Now().UTC()
	return updateTrackedTask(task, retries, "failed", taskErr, &finishedAt)
}

func (ProjectTaskTracker) OnTaskSucceeded(task dto.VideoTaskMessage, retries int) error {
	finishedAt := time.Now().UTC()
	return updateTrackedTask(task, retries, "succeeded", nil, &finishedAt)
}

func updateTrackedTask(task dto.VideoTaskMessage, retries int, status string, taskErr error, finishedAt *time.Time) error {
	store, err := persistence.DefaultStore()
	if err != nil {
		return err
	}

	contentType := taskContentType(task)
	projectID := taskProjectID(task)
	if projectID == "" || contentType == "" {
		return nil
	}

	payloadJSON, err := json.Marshal(task.Payload)
	if err != nil {
		return err
	}

	lastError := ""
	if taskErr != nil {
		lastError = taskErr.Error()
	}

	projectStatus := status
	if status == "succeeded" && task.TaskType != "podcast.page.persist.v1" {
		projectStatus = "running"
	}

	if err := store.UpsertVideoProject(persistence.VideoProjectUpsert{
		ProjectID:       projectID,
		ContentType:     contentType,
		Language:        taskLanguage(task),
		Title:           taskTitle(task),
		Status:          projectStatus,
		CurrentStage:    taskStage(task.TaskType),
		CurrentTaskType: task.TaskType,
		LastError:       lastError,
		RequestPayload:  rootRequestPayload(task, payloadJSON),
		FinishedAt:      finishedAtIfTerminal(status, task.TaskType, finishedAt),
	}); err != nil {
		return err
	}

	return store.UpsertTaskRun(persistence.TaskRunUpsert{
		ProjectID:    projectID,
		TaskID:       task.TaskID,
		TaskType:     task.TaskType,
		Stage:        taskStage(task.TaskType),
		Status:       status,
		RetryCount:   retries,
		Payload:      payloadJSON,
		ErrorMessage: lastError,
		StartedAt:    time.Now().UTC(),
		FinishedAt:   finishedAt,
	})
}

func finishedAtIfTerminal(status string, taskType string, finishedAt *time.Time) *time.Time {
	if status == "failed" {
		return finishedAt
	}
	if status == "succeeded" && taskType == "podcast.page.persist.v1" {
		return finishedAt
	}
	return nil
}

func taskContentType(task dto.VideoTaskMessage) string {
	if task.Payload != nil {
		if raw := strings.TrimSpace(helpers.GetString(task.Payload, "content_type")); raw != "" {
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
	case "podcast.compose.v1":
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

func taskLanguage(task dto.VideoTaskMessage) string {
	if task.Payload == nil {
		return ""
	}
	return strings.TrimSpace(helpers.GetString(task.Payload, "lang"))
}

func taskTitle(task dto.VideoTaskMessage) string {
	if task.Payload == nil {
		return ""
	}
	title := strings.TrimSpace(helpers.GetString(task.Payload, "title"))
	if title != "" {
		return title
	}
	scriptFilename := strings.TrimSpace(helpers.GetString(task.Payload, "script_filename"))
	if scriptFilename != "" {
		return strings.TrimSuffix(scriptFilename, filepathExt(scriptFilename))
	}
	return ""
}

func rootRequestPayload(task dto.VideoTaskMessage, payloadJSON []byte) json.RawMessage {
	switch task.TaskType {
	case "podcast.audio.generate.v1", "plan.v1":
		return payloadJSON
	default:
		return nil
	}
}

func filepathExt(name string) string {
	idx := strings.LastIndex(name, ".")
	if idx < 0 {
		return ""
	}
	return name[idx:]
}

func trackedProjectMetadata(videoURL string, scriptPageID *uint64, scriptPageSlug string, youtubeVideoID string, youtubeVideoURL string) json.RawMessage {
	payload, _ := json.Marshal(map[string]interface{}{
		"video_url":         videoURL,
		"script_page_id":    scriptPageID,
		"script_page_slug":  scriptPageSlug,
		"youtube_video_id":  youtubeVideoID,
		"youtube_video_url": youtubeVideoURL,
	})
	return payload
}

func UpdatePodcastProjectPublication(projectID string, pageID uint64, pageSlug string, videoURL string, youtubeVideoID string, youtubeVideoURL string) error {
	store, err := persistence.DefaultStore()
	if err != nil {
		return err
	}

	now := time.Now().UTC()
	return store.UpsertVideoProject(persistence.VideoProjectUpsert{
		ProjectID:       projectID,
		ContentType:     "podcast",
		Status:          "succeeded",
		CurrentStage:    "script_persist",
		CurrentTaskType: "podcast.page.persist.v1",
		LastError:       "",
		Metadata:        trackedProjectMetadata(videoURL, &pageID, pageSlug, youtubeVideoID, youtubeVideoURL),
		VideoURL:        videoURL,
		YouTubeVideoID:  youtubeVideoID,
		YouTubeVideoURL: youtubeVideoURL,
		ScriptPageID:    &pageID,
		ScriptPageSlug:  pageSlug,
		FinishedAt:      &now,
	})
}

func UpdatePodcastProjectUpload(projectID string, videoURL string, youtubeVideoID string, youtubeVideoURL string) error {
	store, err := persistence.DefaultStore()
	if err != nil {
		return err
	}

	return store.UpsertVideoProject(persistence.VideoProjectUpsert{
		ProjectID:       projectID,
		ContentType:     "podcast",
		Status:          "running",
		CurrentStage:    "upload",
		CurrentTaskType: "upload.v1",
		Metadata:        trackedProjectMetadata(videoURL, nil, "", youtubeVideoID, youtubeVideoURL),
		VideoURL:        videoURL,
		YouTubeVideoID:  youtubeVideoID,
		YouTubeVideoURL: youtubeVideoURL,
	})
}
