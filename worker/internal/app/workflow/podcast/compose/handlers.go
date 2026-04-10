package podcast_compose

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"worker/internal/app/task"
	dto "worker/services/podcast/model"
	podcastcomposeservice "worker/services/podcast/compose"

	amqp "github.com/rabbitmq/amqp091-go"
)

func HandleComposeRender(ctx context.Context, ch *amqp.Channel, msg task.VideoTaskMessage) error {
	payload, err := decodePayload(msg.Payload)
	if err != nil {
		return err
	}
	if strings.TrimSpace(payload.ProjectID) == "" {
		return fmt.Errorf("project_id is required")
	}
	if strings.TrimSpace(payload.Lang) == "" {
		return fmt.Errorf("lang is required")
	}

	_, err = podcastcomposeservice.Render(ctx, podcastcomposeservice.ComposeInput{
		ProjectID:      payload.ProjectID,
		Language:       payload.Lang,
		BgImgFilenames: payload.BgImgFilenames,
		Resolution:     payload.Resolution,
		DesignStyle:    payload.DesignStyle,
	})
	if err != nil {
		return err
	}
	return publishFinalizeTask(ch, payload)
}

func HandleComposeFinalize(ctx context.Context, ch *amqp.Channel, msg task.VideoTaskMessage) error {
	payload, err := decodePayload(msg.Payload)
	if err != nil {
		return err
	}
	return finalizeAndUpload(ctx, ch, payload)
}

func finalizeAndUpload(ctx context.Context, ch *amqp.Channel, payload dto.PodcastComposePayload) error {
	result, err := podcastcomposeservice.Finalize(ctx, composeInputFromPayload(payload))
	if err != nil {
		return err
	}
	log.Printf("🎙️ podcast compose done project_id=%s final=%s", payload.ProjectID, result.FinalVideoPath)
	return publishUploadTask(ch, payload, result.FinalVideoPath)
}

func composeInputFromPayload(payload dto.PodcastComposePayload) podcastcomposeservice.ComposeInput {
	return podcastcomposeservice.ComposeInput{
		ProjectID:      payload.ProjectID,
		Language:       payload.Lang,
		BgImgFilenames: payload.BgImgFilenames,
		Resolution:     payload.Resolution,
		DesignStyle:    payload.DesignStyle,
	}
}

func publishFinalizeTask(ch *amqp.Channel, payload dto.PodcastComposePayload) error {
	return task.PublishTask(ch, "podcast.compose.finalize.v1", map[string]interface{}{
		"content_type":     "podcast",
		"project_id":       payload.ProjectID,
		"lang":             payload.Lang,
		"title":            payload.Title,
		"bg_img_filenames": payload.BgImgFilenames,
		"target_platform":  payload.TargetPlatform,
		"aspect_ratio":     payload.AspectRatio,
		"resolution":       payload.Resolution,
		"design_style":     payload.DesignStyle,
	})
}

func publishUploadTask(ch *amqp.Channel, payload dto.PodcastComposePayload, _ string) error {
	return task.PublishTask(ch, "upload.v1", map[string]interface{}{
		"project_id":   payload.ProjectID,
		"content_type": "podcast",
		"lang":         payload.Lang,
		"title":        payload.Title,
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
