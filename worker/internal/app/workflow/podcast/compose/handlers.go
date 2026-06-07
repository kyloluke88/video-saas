package podcast_compose

import (
	"context"
	"fmt"
	"strings"

	"worker/internal/app/task"
	podcastpipeline "worker/internal/app/workflow/podcast/pipeline"
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
	if shouldInvalidate(string(podcastpipeline.StageRender), payload.StartFrom) {
		if err := podcastpipeline.InvalidateOutputs(payload.ProjectID, payload.TTSType, payload.StartFrom); err != nil {
			return err
		}
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
	return publishNextPodcastTaskFromComposePayload(ch, payload, string(podcastpipeline.StageRender))
}

func HandleComposeFinalize(ctx context.Context, ch *amqp.Channel, msg task.VideoTaskMessage) error {
	payload, err := resolveComposePayload(msg.Payload)
	if err != nil {
		return err
	}
	if shouldInvalidate(string(podcastpipeline.StageFinalize), payload.StartFrom) {
		if err := podcastpipeline.InvalidateOutputs(payload.ProjectID, payload.TTSType, payload.StartFrom); err != nil {
			return err
		}
	}
	return finalizeAndContinue(ctx, ch, payload)
}

func resolveComposePayload(raw map[string]interface{}) (dto.PodcastComposePayload, error) {
	payload, err := podcastpipeline.DecodeComposePayload(raw)
	if err != nil {
		return dto.PodcastComposePayload{}, err
	}
	if payload.RunMode != 0 && payload.RunMode != 1 {
		return dto.PodcastComposePayload{}, services.NonRetryableError{Err: fmt.Errorf("podcast.compose only supports run_mode 0 or 1")}
	}
	payload, err = podcastpipeline.ResolveComposePayload(payload)
	if err != nil {
		return dto.PodcastComposePayload{}, err
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
	return publishNextPodcastTaskFromComposePayload(ch, payload, string(podcastpipeline.StageFinalize))
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

func publishNextPodcastTaskFromComposePayload(ch *amqp.Channel, payload dto.PodcastComposePayload, currentStage string) error {
	nextStage, ok, err := podcastpipeline.NextStage(normalizePodcastTTSType(payload.TTSType), currentStage, payload.StopAt)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	taskType, err := podcastpipeline.TaskTypeForStage(normalizePodcastTTSType(payload.TTSType), podcastpipeline.Stage(nextStage))
	if err != nil {
		return err
	}
	return task.PublishTask(ch, taskType, buildPodcastComposeTaskPayload(payload))
}

func buildPodcastComposeTaskPayload(payload dto.PodcastComposePayload) map[string]interface{} {
	out := map[string]interface{}{
		"content_type": "podcast",
		"project_id":   strings.TrimSpace(payload.ProjectID),
		"run_mode":     payload.RunMode,
		"tts_type":     normalizePodcastTTSType(payload.TTSType),
		"start_from":   strings.TrimSpace(payload.StartFrom),
	}
	if stopAt := strings.TrimSpace(payload.StopAt); stopAt != "" {
		out["stop_at"] = stopAt
	}
	if lang := strings.TrimSpace(payload.Lang); lang != "" {
		out["lang"] = lang
	}
	if len(payload.BgImgFilenames) > 0 {
		out["bg_img_filenames"] = compactNonEmptyStrings(payload.BgImgFilenames)
	}
	if platform := strings.TrimSpace(payload.TargetPlatform); platform != "" {
		out["target_platform"] = platform
	}
	if aspect := strings.TrimSpace(payload.AspectRatio); aspect != "" {
		out["aspect_ratio"] = aspect
	}
	if resolution := strings.TrimSpace(payload.Resolution); resolution != "" {
		out["resolution"] = resolution
	}
	if designStyle := normalizePodcastDesignStyle(payload.DesignStyle); designStyle > 0 {
		out["design_style"] = designStyle
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
	return podcastpipeline.NormalizeTTSType(value)
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

func shouldInvalidate(currentStage string, startFrom string) bool {
	return strings.TrimSpace(currentStage) == strings.TrimSpace(startFrom)
}
