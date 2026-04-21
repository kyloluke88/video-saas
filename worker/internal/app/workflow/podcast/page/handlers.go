package podcast_page

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"worker/internal/app/task"
	podcastreplay "worker/internal/app/workflow/podcast/replay"
	"worker/internal/workspace"
	services "worker/services"
	podcastexportservice "worker/services/podcast/export"
	podcastpageservice "worker/services/podcast/page"

	amqp "github.com/rabbitmq/amqp091-go"
)

type persistPayload struct {
	ProjectID       string   `json:"project_id"`
	SourceProjectID string   `json:"source_project_id,omitempty"`
	RunMode         int      `json:"run_mode,omitempty"`
	TTSType         int      `json:"tts_type,omitempty"`
	SpecifyTasks    []string `json:"specify_tasks,omitempty"`
	Title           string   `json:"title,omitempty"`
	VideoURL        string   `json:"video_url,omitempty"`
	YouTubeVideoID  string   `json:"youtube_video_id,omitempty"`
	YouTubeVideoURL string   `json:"youtube_video_url,omitempty"`
}

func HandlePersist(_ context.Context, ch *amqp.Channel, msg task.VideoTaskMessage) error {
	payload, err := decodePayload(msg.Payload)
	if err != nil {
		return err
	}
	if strings.TrimSpace(payload.ProjectID) == "" {
		return fmt.Errorf("project_id is required")
	}

	if payload.RunMode != 0 && payload.RunMode != 1 {
		return services.NonRetryableError{Err: fmt.Errorf("podcast.page.persist.v1 only supports run_mode 0 or 1")}
	}

	if strings.TrimSpace(payload.SourceProjectID) != "" || payload.RunMode == 1 {
		if payload.RunMode != 1 {
			return services.NonRetryableError{Err: fmt.Errorf("persist replay entry requires run_mode=1")}
		}
		normalizedTasks, err := podcastreplay.ValidateSpecifyTasks(payload.TTSType, payload.RunMode, payload.SpecifyTasks)
		if err != nil {
			return err
		}
		payload.SpecifyTasks = normalizedTasks
		if err := bootstrapReplayPageProject(payload); err != nil {
			return err
		}
	}

	source, err := podcastpageservice.BuildPageSource(podcastpageservice.PersistInput{
		ProjectID:       payload.ProjectID,
		VideoURL:        payload.VideoURL,
		YouTubeVideoID:  payload.YouTubeVideoID,
		YouTubeVideoURL: payload.YouTubeVideoURL,
	})
	if err != nil {
		return err
	}

	if _, err := podcastpageservice.PersistSource(source); err != nil {
		return err
	}

	if _, err := podcastexportservice.GenerateFromPageSource(source); err != nil {
		return err
	}

	if err := task.MarkPodcastProjectPersisted(payload.ProjectID); err != nil {
		return err
	}
	return publishNextPodcastTaskFromPersistPayload(ch, payload)
}

func bootstrapReplayPageProject(payload persistPayload) error {
	if err := podcastreplay.EnsureReplayProjectDirForProject(payload.ProjectID, payload.SourceProjectID); err != nil {
		return err
	}
	patch := buildReplayRequestPayloadPatch(payload)
	merged, err := workspace.LoadRequestPayloadMap(payload.ProjectID)
	if err != nil {
		return err
	}
	for key, value := range patch {
		merged[key] = value
	}
	return workspace.WriteRequestPayload(payload.ProjectID, merged)
}

func buildReplayRequestPayloadPatch(payload persistPayload) map[string]interface{} {
	patch := map[string]interface{}{
		"run_mode": 1,
	}
	if sourceProjectID := strings.TrimSpace(payload.SourceProjectID); sourceProjectID != "" {
		patch["source_project_id"] = sourceProjectID
	}
	if tasks := compactNonEmptyStrings(payload.SpecifyTasks); len(tasks) > 0 {
		patch["specify_tasks"] = tasks
	}
	return patch
}

func decodePayload(raw map[string]interface{}) (persistPayload, error) {
	data, err := json.Marshal(raw)
	if err != nil {
		return persistPayload{}, err
	}
	var payload persistPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return persistPayload{}, err
	}
	return payload, nil
}

func publishNextPodcastTaskFromPersistPayload(ch *amqp.Channel, payload persistPayload) error {
	nextStage, ok, err := podcastreplay.NextPodcastStage(normalizePodcastTTSType(payload.TTSType), string(podcastreplay.PodcastStagePersist), payload.SpecifyTasks)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	taskType, err := podcastreplay.PodcastTaskTypeForStage(normalizePodcastTTSType(payload.TTSType), podcastreplay.PodcastStage(nextStage))
	if err != nil {
		return err
	}
	if taskType != "upload.v1" {
		return task.PublishTask(ch, taskType, buildPodcastPersistTaskPayload(payload))
	}
	return task.PublishTask(ch, taskType, buildPodcastPersistTaskPayload(payload))
}

func buildPodcastPersistTaskPayload(payload persistPayload) map[string]interface{} {
	out := map[string]interface{}{
		"content_type": "podcast",
		"project_id":   strings.TrimSpace(payload.ProjectID),
		"run_mode":     payload.RunMode,
		"tts_type":     normalizePodcastTTSType(payload.TTSType),
	}
	if sourceProjectID := strings.TrimSpace(payload.SourceProjectID); sourceProjectID != "" {
		out["source_project_id"] = sourceProjectID
	}
	if tasks := compactNonEmptyStrings(payload.SpecifyTasks); len(tasks) > 0 && payload.RunMode == 1 {
		out["specify_tasks"] = tasks
	}
	if title := strings.TrimSpace(payload.Title); title != "" {
		out["title"] = title
	}
	if videoURL := strings.TrimSpace(payload.VideoURL); videoURL != "" {
		out["video_url"] = videoURL
	}
	if youtubeVideoID := strings.TrimSpace(payload.YouTubeVideoID); youtubeVideoID != "" {
		out["youtube_video_id"] = youtubeVideoID
	}
	if youtubeVideoURL := strings.TrimSpace(payload.YouTubeVideoURL); youtubeVideoURL != "" {
		out["youtube_video_url"] = youtubeVideoURL
	}
	return out
}

func normalizePodcastTTSType(value int) int {
	if value == 2 {
		return 2
	}
	return 1
}

func compactNonEmptyStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}
