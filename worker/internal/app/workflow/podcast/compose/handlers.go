package podcast_compose

import (
	"context"
	"encoding/json"
	"fmt"
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
	return publishNextPodcastTaskFromComposePayload(ch, payload, string(podcastreplay.PodcastStageRender))
}

func HandleComposeFinalize(ctx context.Context, ch *amqp.Channel, msg task.VideoTaskMessage) error {
	payload, err := resolveComposePayload(msg.Payload)
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
	if payload.RunMode != 0 && payload.RunMode != 1 {
		return dto.PodcastComposePayload{}, services.NonRetryableError{Err: fmt.Errorf("podcast.compose only supports run_mode 0 or 1")}
	}
	if payload.RunMode == 1 || strings.TrimSpace(payload.SourceProjectID) != "" {
		if payload.RunMode != 1 {
			return dto.PodcastComposePayload{}, services.NonRetryableError{Err: fmt.Errorf("compose replay entry requires run_mode=1")}
		}
		if err := podcastreplay.EnsureReplayProjectDirForProject(payload.ProjectID, payload.SourceProjectID); err != nil {
			return dto.PodcastComposePayload{}, err
		}
		normalizedTasks, err := podcastreplay.ValidateSpecifyTasks(payload.TTSType, payload.RunMode, payload.SpecifyTasks)
		if err != nil {
			return dto.PodcastComposePayload{}, err
		}
		payload.SpecifyTasks = normalizedTasks
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
	_, err := podcastcomposeservice.Finalize(ctx, composeInputFromPayload(payload))
	if err != nil {
		return err
	}
	return publishNextPodcastTaskFromComposePayload(ch, payload, string(podcastreplay.PodcastStageFinalize))
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
	return task.PublishTask(ch, "podcast.compose.finalize.v1", buildPodcastComposeTaskPayload(payload))
}

func publishPersistTask(ch *amqp.Channel, payload dto.PodcastComposePayload) error {
	return task.PublishTask(ch, "podcast.page.persist.v1", buildPodcastComposeTaskPayload(payload))
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

func publishNextPodcastTaskFromComposePayload(ch *amqp.Channel, payload dto.PodcastComposePayload, currentStage string) error {
	nextStage, ok, err := podcastreplay.NextPodcastStage(normalizePodcastTTSType(payload.TTSType), currentStage, payload.SpecifyTasks)
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
	switch taskType {
	case "podcast.compose.finalize.v1":
		return publishFinalizeTask(ch, payload)
	case "podcast.page.persist.v1":
		return publishPersistTask(ch, payload)
	case "upload.v1":
		return task.PublishTask(ch, taskType, buildPodcastComposeTaskPayload(payload))
	default:
		return task.PublishTask(ch, taskType, buildPodcastComposeTaskPayload(payload))
	}
}

func buildPodcastComposeTaskPayload(payload dto.PodcastComposePayload) map[string]interface{} {
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
	if lang := strings.TrimSpace(payload.Lang); lang != "" {
		out["lang"] = lang
	}
	if title := strings.TrimSpace(payload.Title); title != "" {
		out["title"] = title
	}
	if len(payload.BgImgFilenames) > 0 {
		out["bg_img_filenames"] = compactNonEmptyStrings(payload.BgImgFilenames)
	}
	if targetPlatform := strings.TrimSpace(payload.TargetPlatform); targetPlatform != "" {
		out["target_platform"] = targetPlatform
	}
	if aspectRatio := strings.TrimSpace(payload.AspectRatio); aspectRatio != "" {
		out["aspect_ratio"] = aspectRatio
	}
	if resolution := strings.TrimSpace(payload.Resolution); resolution != "" {
		out["resolution"] = resolution
	}
	if designStyle := normalizePodcastDesignStyle(payload.DesignStyle); designStyle > 0 {
		out["design_style"] = designStyle
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

func normalizePodcastDesignStyle(value int) int {
	if value == 2 {
		return 2
	}
	return 1
}
