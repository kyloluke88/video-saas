package podcast_page

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"worker/internal/app/task"
	podcastreplay "worker/internal/app/workflow/podcast/replay"
	services "worker/services"
	podcastexportservice "worker/services/podcast/export"
	podcastpageservice "worker/services/podcast/page"

	amqp "github.com/rabbitmq/amqp091-go"
)

type persistPayload struct {
	ProjectID       string `json:"project_id"`
	SourceProjectID string `json:"source_project_id,omitempty"`
	RunMode         int    `json:"run_mode,omitempty"`
	OnlyCurrentStep int    `json:"only_current_step,omitempty"`
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

	if strings.TrimSpace(payload.SourceProjectID) != "" {
		if payload.RunMode != 3 {
			return services.NonRetryableError{Err: fmt.Errorf("persist replay entry requires run_mode=3")}
		}
		if err := bootstrapReplayPageProject(msg.Payload); err != nil {
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
	if payload.OnlyCurrentStep == 1 {
		return nil
	}
	return task.PublishTask(ch, "upload.v1", map[string]interface{}{
		"project_id":        payload.ProjectID,
		"content_type":      "podcast",
		"run_mode":          payload.RunMode,
		"only_current_step": payload.OnlyCurrentStep,
	})
}

func bootstrapReplayPageProject(raw map[string]interface{}) error {
	current, err := podcastreplay.DecodeGeneratePayload(raw)
	if err != nil {
		return err
	}
	_, err = podcastreplay.PrepareGeneratePayload(current, raw)
	return err
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
