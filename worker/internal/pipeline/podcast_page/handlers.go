package podcast_page

import (
	"encoding/json"
	"fmt"
	"strings"

	"worker/internal/dto"
	"worker/internal/pipeline"
	podcastpageservice "worker/services/podcast_page_service"

	amqp "github.com/rabbitmq/amqp091-go"
)

type persistPayload struct {
	ProjectID       string `json:"project_id"`
	VideoURL        string `json:"video_url,omitempty"`
	YouTubeVideoID  string `json:"youtube_video_id,omitempty"`
	YouTubeVideoURL string `json:"youtube_video_url,omitempty"`
}

func HandlePersist(_ *amqp.Channel, task dto.VideoTaskMessage) error {
	payload, err := decodePayload(task.Payload)
	if err != nil {
		return err
	}
	if strings.TrimSpace(payload.ProjectID) == "" {
		return fmt.Errorf("project_id is required")
	}

	result, err := podcastpageservice.Persist(podcastpageservice.PersistInput{
		ProjectID:       payload.ProjectID,
		VideoURL:        payload.VideoURL,
		YouTubeVideoID:  payload.YouTubeVideoID,
		YouTubeVideoURL: payload.YouTubeVideoURL,
	})
	if err != nil {
		return err
	}

	return pipeline.UpdatePodcastProjectPublication(
		payload.ProjectID,
		result.PageID,
		result.Slug,
		payload.VideoURL,
		payload.YouTubeVideoID,
		payload.YouTubeVideoURL,
	)
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
