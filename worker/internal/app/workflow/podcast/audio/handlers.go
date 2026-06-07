package podcast_audio

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"worker/internal/app/task"
	podcastpipeline "worker/internal/app/workflow/podcast/pipeline"
	services "worker/services"
	podcastaudioservice "worker/services/podcast/audio"
	dto "worker/services/podcast/model"

	amqp "github.com/rabbitmq/amqp091-go"
)

func HandleGenerate(ctx context.Context, ch *amqp.Channel, msg task.VideoTaskMessage) error {
	payload, err := decodePayload(msg.Payload)
	if err != nil {
		return err
	}
	if payload.RunMode != 0 && payload.RunMode != 1 {
		return services.NonRetryableError{Err: fmt.Errorf("podcast.audio.generate.v1 only supports run_mode 0 or 1")}
	}

	payload, err = podcastpipeline.ResolveGeneratePayload(payload)
	if err != nil {
		return err
	}
	if err := validateFreshGeneratePayload(payload); err != nil {
		return err
	}
	if shouldInvalidate(string(podcastpipeline.StageGenerate), payload.StartFrom) {
		if err := podcastpipeline.InvalidateOutputs(payload.ProjectID, payload.TTSType, payload.StartFrom); err != nil {
			return err
		}
	}
	if err := podcastpipeline.PersistGeneratePayload(payload); err != nil {
		return err
	}
	return generateAndContinue(ctx, ch, payload)
}

func HandleAlign(ctx context.Context, ch *amqp.Channel, msg task.VideoTaskMessage) error {
	payload, err := decodePayload(msg.Payload)
	if err != nil {
		return err
	}
	if payload.RunMode != 0 && payload.RunMode != 1 {
		return services.NonRetryableError{Err: fmt.Errorf("podcast.audio.align.v1 only supports run_mode 0 or 1")}
	}

	payload, err = podcastpipeline.ResolveGeneratePayload(payload)
	if err != nil {
		return err
	}
	if podcastpipeline.NormalizeTTSType(payload.TTSType) != 1 {
		return services.NonRetryableError{Err: fmt.Errorf("align task is only valid for google tts projects")}
	}
	if shouldInvalidate(string(podcastpipeline.StageAlign), payload.StartFrom) {
		if err := podcastpipeline.InvalidateOutputs(payload.ProjectID, payload.TTSType, payload.StartFrom); err != nil {
			return err
		}
	}
	if err := podcastpipeline.PersistGeneratePayload(payload); err != nil {
		return err
	}
	return alignAndContinue(ctx, ch, payload)
}

func validPodcastLanguage(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "zh", "ja":
		return true
	default:
		return false
	}
}

func validPodcastTTSType(value int) bool {
	switch value {
	case 1, 2:
		return true
	default:
		return false
	}
}

func normalizePodcastTTSType(value int) int {
	return podcastpipeline.NormalizeTTSType(value)
}

func validPodcastDesignStyle(value int) bool {
	switch value {
	case 1, 2:
		return true
	default:
		return false
	}
}

func normalizePodcastDesignStyle(value int) int {
	if value == 2 {
		return 2
	}
	return 1
}

func decodePayload(raw map[string]interface{}) (dto.PodcastAudioGeneratePayload, error) {
	data, err := json.Marshal(raw)
	if err != nil {
		return dto.PodcastAudioGeneratePayload{}, err
	}
	var payload dto.PodcastAudioGeneratePayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return dto.PodcastAudioGeneratePayload{}, err
	}
	return payload, nil
}

func validateFreshGeneratePayload(payload dto.PodcastAudioGeneratePayload) error {
	if strings.TrimSpace(payload.ProjectID) == "" {
		return fmt.Errorf("project_id is required")
	}
	if !validPodcastLanguage(payload.Lang) {
		return fmt.Errorf("lang must be zh or ja")
	}
	if !validPodcastTTSType(payload.TTSType) {
		return fmt.Errorf("tts_type must be 1 or 2")
	}
	if strings.TrimSpace(payload.ScriptFilename) == "" {
		return fmt.Errorf("script_filename is required")
	}
	if len(compactNonEmptyStrings(payload.BgImgFilenames)) == 0 {
		return fmt.Errorf("bg_img_filenames is required")
	}
	if payload.DesignStyle != 0 && !validPodcastDesignStyle(payload.DesignStyle) {
		return fmt.Errorf("design_style must be 1 or 2")
	}
	return nil
}

func generateAndContinue(ctx context.Context, ch *amqp.Channel, payload dto.PodcastAudioGeneratePayload) error {
	ttsType := normalizePodcastTTSType(payload.TTSType)
	isMultiple := normalizePodcastIsMultiple(payload.IsMultiple)
	if ttsType == 1 {
		if err := podcastaudioservice.GenerateGoogleAudio(ctx, podcastaudioservice.GenerateInput{
			ProjectID:      payload.ProjectID,
			Language:       payload.Lang,
			TTSType:        ttsType,
			IsMultiple:     isMultiple,
			Seed:           payload.Seed,
			BlockNums:      compactPositiveInts(payload.BlockNums),
			ScriptFilename: payload.ScriptFilename,
		}); err != nil {
			return err
		}
		return publishNextPodcastTaskFromGeneratePayload(ch, payload, string(podcastpipeline.StageGenerate))
	}

	_, err := podcastaudioservice.Generate(ctx, podcastaudioservice.GenerateInput{
		ProjectID:      payload.ProjectID,
		Language:       payload.Lang,
		TTSType:        ttsType,
		IsMultiple:     isMultiple,
		Seed:           payload.Seed,
		BlockNums:      compactPositiveInts(payload.BlockNums),
		ScriptFilename: payload.ScriptFilename,
	})
	if err != nil {
		return err
	}
	return publishNextPodcastTaskFromGeneratePayload(ch, payload, string(podcastpipeline.StageGenerate))
}

func alignAndContinue(ctx context.Context, ch *amqp.Channel, payload dto.PodcastAudioGeneratePayload) error {
	isMultiple := normalizePodcastIsMultiple(payload.IsMultiple)
	_, err := podcastaudioservice.AlignGoogle(ctx, podcastaudioservice.AlignInput{
		ProjectID:  payload.ProjectID,
		Language:   payload.Lang,
		IsMultiple: isMultiple,
		BlockNums:  compactPositiveInts(payload.BlockNums),
	})
	if err != nil {
		return err
	}
	return publishNextPodcastTaskFromGeneratePayload(ch, payload, string(podcastpipeline.StageAlign))
}

func publishNextPodcastTaskFromGeneratePayload(ch *amqp.Channel, payload dto.PodcastAudioGeneratePayload, currentStage string) error {
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
	return publishPodcastAudioTask(ch, taskType, payload)
}

func publishPodcastAudioTask(ch *amqp.Channel, taskType string, payload dto.PodcastAudioGeneratePayload) error {
	return task.PublishTask(ch, taskType, buildPodcastAudioTaskPayload(payload))
}

func normalizePodcastIsMultiple(value *int) int {
	if value == nil {
		return 1
	}
	if *value == 0 {
		return 0
	}
	return 1
}

func shouldInvalidate(currentStage string, startFrom string) bool {
	return strings.TrimSpace(currentStage) == strings.TrimSpace(startFrom)
}

func firstBackgroundName(backgrounds []string) string {
	for _, value := range backgrounds {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func compactPositiveInts(values []int) []int {
	seen := make(map[int]struct{})
	out := make([]int, 0, len(values))
	for _, value := range values {
		if value <= 0 {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
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

func buildPodcastAudioTaskPayload(payload dto.PodcastAudioGeneratePayload) map[string]interface{} {
	out := map[string]interface{}{
		"content_type": "podcast",
		"project_id":   strings.TrimSpace(payload.ProjectID),
		"run_mode":     payload.RunMode,
		"tts_type":     normalizePodcastTTSType(payload.TTSType),
		"lang":         strings.TrimSpace(payload.Lang),
		"start_from":   strings.TrimSpace(payload.StartFrom),
	}
	if stopAt := strings.TrimSpace(payload.StopAt); stopAt != "" {
		out["stop_at"] = stopAt
	}
	if scriptFile := strings.TrimSpace(payload.ScriptFilename); scriptFile != "" {
		out["script_filename"] = scriptFile
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
	if seed := payload.Seed; seed > 0 {
		out["seed"] = seed
	}
	if isMultiple, ok := payloadOptionalInt(payload.IsMultiple); ok {
		out["is_multiple"] = isMultiple
	} else if normalizePodcastTTSType(payload.TTSType) == 1 {
		out["is_multiple"] = 1
	}
	if blockNums := compactPositiveInts(payload.BlockNums); len(blockNums) > 0 {
		out["block_nums"] = blockNums
	}
	if backgrounds := compactNonEmptyStrings(payload.BgImgFilenames); len(backgrounds) > 0 {
		out["bg_img_filenames"] = backgrounds
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

func payloadOptionalInt(value *int) (int, bool) {
	if value == nil {
		return 0, false
	}
	return *value, true
}
