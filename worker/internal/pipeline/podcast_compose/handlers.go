package podcast_compose

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"worker/internal/dto"
	"worker/internal/pipeline"
	podcastcomposeservice "worker/services/podcast_compose_service"

	amqp "github.com/rabbitmq/amqp091-go"
)

func HandleCompose(ch *amqp.Channel, task dto.VideoTaskMessage) error {
	payload, err := decodePayload(task.Payload)
	if err != nil {
		return err
	}
	if strings.TrimSpace(payload.ProjectID) == "" {
		return fmt.Errorf("project_id is required")
	}
	if strings.TrimSpace(payload.Lang) == "" {
		return fmt.Errorf("lang is required")
	}

	result, err := podcastcomposeservice.Compose(podcastcomposeservice.ComposeInput{
		ProjectID:      payload.ProjectID,
		Language:       payload.Lang,
		BgImgFilenames: payload.BgImgFilenames,
		Resolution:     payload.Resolution,
		DesignStyle:    payload.DesignStyle,
	})
	if err != nil {
		return err
	}

	log.Printf("🎙️ podcast compose done project_id=%s final=%s", payload.ProjectID, result.FinalVideoPath)
	return pipeline.PublishTask(ch, "upload.v1", map[string]interface{}{
		"project_id": payload.ProjectID,
		"file_path":  result.FinalVideoPath,
	})
}

func decodePayload(raw map[string]interface{}) (dto.PodcastComposePayload, error) {
	data, err := json.Marshal(raw)
	if err != nil {
		return dto.PodcastComposePayload{}, err
	}
	var payload dto.PodcastComposePayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return dto.PodcastComposePayload{}, err
	}
	return payload, nil
}
