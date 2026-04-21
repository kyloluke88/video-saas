package podcast_audio

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"worker/internal/app/task"
	podcastreplay "worker/internal/app/workflow/podcast/replay"
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
	switch payload.RunMode {
	case 1:
		return handleRunModeReplay(ctx, ch, payload, msg.Payload)
	case 0:
		return handleRunModeFresh(ctx, ch, payload)
	default:
		return services.NonRetryableError{Err: fmt.Errorf("podcast.audio.generate.v1 only supports run_mode 0 or 1")}
	}
}

func HandleAlign(ctx context.Context, ch *amqp.Channel, msg task.VideoTaskMessage) error {
	payload, err := decodePayload(msg.Payload)
	if err != nil {
		return err
	}
	if payload.RunMode == 1 || strings.TrimSpace(payload.SourceProjectID) != "" {
		if payload.RunMode != 1 {
			return services.NonRetryableError{Err: fmt.Errorf("podcast.audio.align.v1 replay entry requires run_mode=1")}
		}
		replayPayload, err := podcastreplay.PrepareGeneratePayload(payload, msg.Payload)
		if err != nil {
			return err
		}
		if normalizePodcastTTSType(replayPayload.TTSType) != 1 {
			return services.NonRetryableError{Err: fmt.Errorf("align task is only valid for google tts projects")}
		}
		normalizedTasks, err := podcastreplay.ValidateSpecifyTasks(replayPayload.TTSType, replayPayload.RunMode, replayPayload.SpecifyTasks)
		if err != nil {
			return err
		}
		replayPayload.SpecifyTasks = normalizedTasks
		return alignAndContinue(ctx, ch, replayPayload)
	}
	if normalizePodcastTTSType(payload.TTSType) != 1 {
		return services.NonRetryableError{Err: fmt.Errorf("align task is only valid for google tts projects")}
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
	if value == 2 {
		return 2
	}
	return 1
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

func handleRunModeFresh(ctx context.Context, ch *amqp.Channel, payload dto.PodcastAudioGeneratePayload) error {
	if err := validateFreshGeneratePayload(payload); err != nil {
		return err
	}
	if err := podcastreplay.PersistGeneratePayload(payload); err != nil {
		return err
	}
	return generateAndContinue(ctx, ch, payload)
}

func handleRunModeReplay(ctx context.Context, ch *amqp.Channel, payload dto.PodcastAudioGeneratePayload, rawPayload map[string]interface{}) error {
	replayPayload, err := podcastreplay.PrepareGeneratePayload(payload, rawPayload)
	if err != nil {
		return err
	}
	normalizedTasks, err := podcastreplay.ValidateSpecifyTasks(replayPayload.TTSType, replayPayload.RunMode, replayPayload.SpecifyTasks)
	if err != nil {
		return err
	}
	replayPayload.SpecifyTasks = normalizedTasks
	if containsPodcastTask(replayPayload.SpecifyTasks, string(podcastreplay.PodcastStageGenerate)) && len(compactPositiveInts(replayPayload.BlockNums)) == 0 {
		return services.NonRetryableError{Err: fmt.Errorf("block_nums is required when specify_tasks includes generate")}
	}
	return generateAndContinue(ctx, ch, replayPayload)
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
		return publishNextPodcastTaskFromGeneratePayload(ch, payload, string(podcastreplay.PodcastStageGenerate))
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
	return publishNextPodcastTaskFromGeneratePayload(ch, payload, string(podcastreplay.PodcastStageGenerate))
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
	return publishNextPodcastTaskFromGeneratePayload(ch, payload, string(podcastreplay.PodcastStageAlign))
}

func publishNextPodcastTaskFromGeneratePayload(ch *amqp.Channel, payload dto.PodcastAudioGeneratePayload, currentStage string) error {
	currentStageName := strings.TrimSpace(currentStage)
	nextStage, ok, err := podcastreplay.NextPodcastStage(normalizePodcastTTSType(payload.TTSType), currentStageName, payload.SpecifyTasks)
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
	}
	if sourceProjectID := strings.TrimSpace(payload.SourceProjectID); sourceProjectID != "" {
		out["source_project_id"] = sourceProjectID
	}
	if tasks := compactNonEmptyStrings(payload.SpecifyTasks); len(tasks) > 0 && payload.RunMode == 1 {
		out["specify_tasks"] = tasks
	}
	if title := strings.TrimSpace(payload.Title); title != "" {
		out["title"] = title
	}
	if lang := strings.TrimSpace(payload.Lang); lang != "" {
		out["lang"] = lang
	}
	if contentProfile := strings.TrimSpace(payload.ContentProfile); contentProfile != "" {
		out["content_profile"] = contentProfile
	}
	if scriptFilename := strings.TrimSpace(payload.ScriptFilename); scriptFilename != "" {
		out["script_filename"] = scriptFilename
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
	if seed := payload.Seed; seed > 0 {
		out["seed"] = seed
	}
	if len(payload.BlockNums) > 0 {
		out["block_nums"] = compactPositiveInts(payload.BlockNums)
	}
	if len(payload.BgImgFilenames) > 0 {
		out["bg_img_filenames"] = compactNonEmptyStrings(payload.BgImgFilenames)
	}
	if isMultiple := payload.IsMultiple; isMultiple != nil {
		out["is_multiple"] = normalizePodcastIsMultiple(isMultiple)
	} else if normalizePodcastTTSType(payload.TTSType) == 1 {
		out["is_multiple"] = 1
	}
	return out
}

func containsPodcastTask(values []string, target string) bool {
	target = strings.TrimSpace(target)
	for _, raw := range values {
		if strings.TrimSpace(raw) == target {
			return true
		}
	}
	return false
}
