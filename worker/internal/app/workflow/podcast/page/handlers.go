package podcast_page

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"worker/internal/app/task"
	podcastaudiopipeline "worker/internal/app/workflow/podcast/audio"
	"worker/internal/workspace"
	dto "worker/services/podcast/model"
	podcastexportservice "worker/services/podcast/export"
	podcastpageservice "worker/services/podcast/page"

	amqp "github.com/rabbitmq/amqp091-go"
)

type persistPayload struct {
	ProjectID       string `json:"project_id"`
	SourceProjectID string `json:"source_project_id,omitempty"`
	VideoURL        string `json:"video_url,omitempty"`
	YouTubeVideoID  string `json:"youtube_video_id,omitempty"`
	YouTubeVideoURL string `json:"youtube_video_url,omitempty"`
}

func HandlePersist(_ context.Context, ch *amqp.Channel, msg task.VideoTaskMessage) error {
	payload, err := decodePayload(msg.Payload)
	if err != nil {
		return err
	}
	if strings.TrimSpace(payload.ProjectID) == "" {
		return fmt.Errorf("project_id is required")
	}

	if payloadRequiresReplayBootstrap(payload) {
		if err := bootstrapReplayPageProject(msg.Payload, payload); err != nil {
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
	return task.PublishTask(ch, "upload.v1", map[string]interface{}{
		"project_id":   payload.ProjectID,
		"content_type": "podcast",
	})
}

func bootstrapReplayPageProject(raw map[string]interface{}, payload persistPayload) error {
	sourceProjectID, err := resolveReplayPageSourceProjectID(payload)
	if err != nil {
		return err
	}
	if err := workspace.EnsureReplayProjectDir(sourceProjectID, payload.ProjectID); err != nil {
		return err
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return err
	}
	var current dto.PodcastAudioGeneratePayload
	if err := json.Unmarshal(data, &current); err != nil {
		return err
	}
	current.ProjectID = payload.ProjectID
	replayPayload, err := podcastaudiopipeline.BuildReplayPayloadForPersist(current)
	if err != nil {
		return err
	}
	return workspace.WriteRequestPayload(payload.ProjectID, replayPayload)
}

func payloadRequiresReplayBootstrap(payload persistPayload) bool {
	if strings.TrimSpace(payload.SourceProjectID) != "" {
		return true
	}
	_, err := workspace.ReplaySourceProjectID(payload.ProjectID)
	return err == nil
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

func resolveReplayPageSourceProjectID(payload persistPayload) (string, error) {
	if sourceProjectID := strings.TrimSpace(payload.SourceProjectID); sourceProjectID != "" {
		return sourceProjectID, nil
	}
	return workspace.ReplaySourceProjectID(payload.ProjectID)
}
