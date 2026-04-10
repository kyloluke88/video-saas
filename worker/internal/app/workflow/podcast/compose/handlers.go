package podcast_compose

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"worker/internal/app/task"
	podcastreplay "worker/internal/app/workflow/podcast/replay"
	services "worker/services"
	podcastcomposeservice "worker/services/podcast/compose"
	dto "worker/services/podcast/model"

	amqp "github.com/rabbitmq/amqp091-go"
)

func HandleComposeRender(ctx context.Context, ch *amqp.Channel, msg task.VideoTaskMessage) error {
	payload, err := resolveComposePayload(msg.Payload)
	if err != nil {
		return err
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
	if shouldStopAfterCurrentStep(payload.OnlyCurrentStep) {
		return nil
	}
	return publishFinalizeTask(ch, payload)
}

func HandleComposeFinalize(ctx context.Context, ch *amqp.Channel, msg task.VideoTaskMessage) error {
	payload, err := decodePayload(msg.Payload)
	if err != nil {
		return err
	}
	return finalizeAndContinue(ctx, ch, payload)
}

func resolveComposePayload(raw map[string]interface{}) (dto.PodcastComposePayload, error) {
	payload, err := decodePayload(raw)
	if err != nil {
		return dto.PodcastComposePayload{}, err
	}
	if strings.TrimSpace(payload.SourceProjectID) != "" {
		if payload.RunMode != 2 {
			return dto.PodcastComposePayload{}, services.NonRetryableError{Err: fmt.Errorf("compose replay entry requires run_mode=2")}
		}
		generatePayload, err := podcastreplay.DecodeGeneratePayload(raw)
		if err != nil {
			return dto.PodcastComposePayload{}, err
		}
		replayPayload, err := podcastreplay.PrepareGeneratePayload(generatePayload)
		if err != nil {
			return dto.PodcastComposePayload{}, err
		}
		return podcastreplay.BuildComposePayloadFromGenerate(replayPayload)
	}
	if strings.TrimSpace(payload.ProjectID) == "" {
		return dto.PodcastComposePayload{}, fmt.Errorf("project_id is required")
	}
	if strings.TrimSpace(payload.Lang) == "" {
		return dto.PodcastComposePayload{}, fmt.Errorf("lang is required")
	}
	if len(payload.BgImgFilenames) == 0 {
		return dto.PodcastComposePayload{}, fmt.Errorf("bg_img_filenames is required")
	}
	return payload, nil
}

func finalizeAndContinue(ctx context.Context, ch *amqp.Channel, payload dto.PodcastComposePayload) error {
	result, err := podcastcomposeservice.Finalize(ctx, composeInputFromPayload(payload))
	if err != nil {
		return err
	}
	log.Printf("🎙️ podcast compose done project_id=%s final=%s", payload.ProjectID, result.FinalVideoPath)
	if shouldStopAfterCurrentStep(payload.OnlyCurrentStep) {
		return nil
	}
	return publishPersistTask(ch, payload)
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
		"content_type":      "podcast",
		"project_id":        payload.ProjectID,
		"lang":              payload.Lang,
		"run_mode":          payload.RunMode,
		"only_current_step": payload.OnlyCurrentStep,
		"title":             payload.Title,
		"bg_img_filenames":  payload.BgImgFilenames,
		"target_platform":   payload.TargetPlatform,
		"aspect_ratio":      payload.AspectRatio,
		"resolution":        payload.Resolution,
		"design_style":      payload.DesignStyle,
	})
}

func publishPersistTask(ch *amqp.Channel, payload dto.PodcastComposePayload) error {
	return task.PublishTask(ch, "podcast.page.persist.v1", map[string]interface{}{
		"content_type":      "podcast",
		"project_id":        payload.ProjectID,
		"run_mode":          payload.RunMode,
		"only_current_step": payload.OnlyCurrentStep,
		"lang":              payload.Lang,
		"title":             payload.Title,
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

func shouldStopAfterCurrentStep(value int) bool {
	return value == 1
}
