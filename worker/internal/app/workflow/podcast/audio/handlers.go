package podcast_audio

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
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
	if payload.SourceProjectID != "" {
		if payload.RunMode != 4 {
			return services.NonRetryableError{Err: fmt.Errorf("replay align entry requires run_mode=4")}
		}
		replayPayload, err := podcastreplay.PrepareGeneratePayload(payload, msg.Payload)
		if err != nil {
			return err
		}
		if normalizePodcastTTSType(replayPayload.TTSType) != 1 {
			return services.NonRetryableError{Err: fmt.Errorf("run_mode=4 is only valid for google tts projects")}
		}
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
	case 0, 1, 2:
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
	sourceProjectID, _ := podcastreplay.ResolveSourceProjectID(payload)
	log.Printf("♻️ podcast run_mode=1 replay source=%s target=%s block_nums=%v background=%s backgrounds=%d design_style=%d resolution=%s project_id=%s",
		sourceProjectID, replayPayload.ProjectID, replayPayload.BlockNums, firstBackgroundName(replayPayload.BgImgFilenames), len(replayPayload.BgImgFilenames), replayPayload.DesignStyle, replayPayload.Resolution, replayPayload.ProjectID)
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
	if ttsType == 1 {
		if err := podcastaudioservice.GenerateGoogleAudio(ctx, podcastaudioservice.GenerateInput{
			ProjectID:      payload.ProjectID,
			Language:       payload.Lang,
			TTSType:        ttsType,
			Seed:           payload.Seed,
			BlockNums:      compactPositiveInts(payload.BlockNums),
			ScriptFilename: payload.ScriptFilename,
		}); err != nil {
			return err
		}
		if shouldStopAfterCurrentStep(payload.OnlyCurrentStep) {
			return nil
		}
		return publishAlignTask(ch, payload)
	}

	_, err := podcastaudioservice.Generate(ctx, podcastaudioservice.GenerateInput{
		ProjectID:      payload.ProjectID,
		Language:       payload.Lang,
		TTSType:        ttsType,
		Seed:           payload.Seed,
		BlockNums:      compactPositiveInts(payload.BlockNums),
		ScriptFilename: payload.ScriptFilename,
	})
	if err != nil {
		return err
	}
	if shouldStopAfterCurrentStep(payload.OnlyCurrentStep) {
		return nil
	}

	composePayload, err := podcastreplay.BuildComposePayloadFromGenerate(payload)
	if err != nil {
		return err
	}
	return publishComposeTask(ch, composePayload)
}

func alignAndContinue(ctx context.Context, ch *amqp.Channel, payload dto.PodcastAudioGeneratePayload) error {
	_, err := podcastaudioservice.AlignGoogle(ctx, podcastaudioservice.AlignInput{
		ProjectID: payload.ProjectID,
		Language:  payload.Lang,
		BlockNums: compactPositiveInts(payload.BlockNums),
	})
	if err != nil {
		return err
	}
	if shouldStopAfterCurrentStep(payload.OnlyCurrentStep) {
		return nil
	}

	composePayload, err := podcastreplay.BuildComposePayloadFromGenerate(payload)
	if err != nil {
		return err
	}
	return publishComposeTask(ch, composePayload)
}

func publishAlignTask(ch *amqp.Channel, payload dto.PodcastAudioGeneratePayload) error {
	alignPayload := map[string]interface{}{
		"content_type":      "podcast",
		"project_id":        payload.ProjectID,
		"lang":              payload.Lang,
		"tts_type":          normalizePodcastTTSType(payload.TTSType),
		"run_mode":          payload.RunMode,
		"only_current_step": payload.OnlyCurrentStep,
		"title":             payload.Title,
		"target_platform":   payload.TargetPlatform,
		"aspect_ratio":      payload.AspectRatio,
		"resolution":        payload.Resolution,
		"design_style":      normalizePodcastDesignStyle(payload.DesignStyle),
	}
	if len(payload.BlockNums) > 0 {
		alignPayload["block_nums"] = compactPositiveInts(payload.BlockNums)
	}
	if len(payload.BgImgFilenames) > 0 {
		alignPayload["bg_img_filenames"] = compactNonEmptyStrings(payload.BgImgFilenames)
	}
	return task.PublishTask(ch, "podcast.audio.align.v1", alignPayload)
}

func publishComposeTask(ch *amqp.Channel, payload dto.PodcastComposePayload) error {
	return task.PublishTask(ch, "podcast.compose.render.v1", map[string]interface{}{
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
		"design_style":      normalizePodcastDesignStyle(payload.DesignStyle),
	})
}

func shouldStopAfterCurrentStep(value int) bool {
	return value == 1
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
