package podcast_page

import (
	"context"
	"fmt"
	"strings"

	"worker/internal/app/task"
	podcastpipeline "worker/internal/app/workflow/podcast/pipeline"
	services "worker/services"
	podcastexportservice "worker/services/podcast/export"
	dto "worker/services/podcast/model"
	podcastpageservice "worker/services/podcast/page"

	amqp "github.com/rabbitmq/amqp091-go"
)

func HandlePersist(_ context.Context, ch *amqp.Channel, msg task.VideoTaskMessage) error {
	payload, err := resolvePersistPayload(msg.Payload)
	if err != nil {
		return err
	}
	if shouldInvalidate(string(podcastpipeline.StagePersist), payload.StartFrom) {
		if err := podcastpipeline.InvalidateOutputs(payload.ProjectID, payload.TTSType, payload.StartFrom); err != nil {
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

func resolvePersistPayload(raw map[string]interface{}) (dto.PodcastComposePayload, error) {
	payload, err := podcastpipeline.DecodeComposePayload(raw)
	if err != nil {
		return dto.PodcastComposePayload{}, err
	}
	if payload.RunMode != 0 && payload.RunMode != 1 {
		return dto.PodcastComposePayload{}, services.NonRetryableError{Err: fmt.Errorf("podcast.page.persist.v1 only supports run_mode 0 or 1")}
	}
	payload, err = podcastpipeline.ResolveComposePayload(payload)
	if err != nil {
		return dto.PodcastComposePayload{}, err
	}
	if strings.TrimSpace(payload.ProjectID) == "" {
		return dto.PodcastComposePayload{}, fmt.Errorf("project_id is required")
	}
	return payload, nil
}

func publishNextPodcastTaskFromPersistPayload(ch *amqp.Channel, payload dto.PodcastComposePayload) error {
	nextStage, ok, err := podcastpipeline.NextStage(normalizePodcastTTSType(payload.TTSType), string(podcastpipeline.StagePersist), payload.StopAt)
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
	return task.PublishTask(ch, taskType, buildPodcastPersistTaskPayload(payload))
}

func buildPodcastPersistTaskPayload(payload dto.PodcastComposePayload) map[string]interface{} {
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
	if videoURL := strings.TrimSpace(payload.VideoURL); videoURL != "" {
		out["video_url"] = videoURL
	}
	if youtubeVideoID := strings.TrimSpace(payload.YouTubeVideoID); youtubeVideoID != "" {
		out["youtube_video_id"] = youtubeVideoID
	}
	if youtubeVideoURL := strings.TrimSpace(payload.YouTubeVideoURL); youtubeVideoURL != "" {
		out["youtube_video_url"] = youtubeVideoURL
	}
	if lang := strings.TrimSpace(payload.Lang); lang != "" {
		out["lang"] = lang
	}
	if backgrounds := compactNonEmptyStrings(payload.BgImgFilenames); len(backgrounds) > 0 {
		out["bg_img_filenames"] = backgrounds
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
