package practical_audio

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"worker/internal/app/task"
	practicalreplay "worker/internal/app/workflow/practical/replay"
	services "worker/services"
	practicalaudioservice "worker/services/practical/audio"
	dto "worker/services/practical/model"

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
		return services.NonRetryableError{Err: fmt.Errorf("practical.audio.generate.v1 only supports run_mode 0 or 1")}
	}
}

func HandleAlign(ctx context.Context, ch *amqp.Channel, msg task.VideoTaskMessage) error {
	payload, err := decodePayload(msg.Payload)
	if err != nil {
		return err
	}
	if payload.RunMode != 0 && payload.RunMode != 1 {
		return services.NonRetryableError{Err: fmt.Errorf("practical.audio.align.v1 only supports run_mode 0 or 1")}
	}
	if payload.RunMode == 1 || strings.TrimSpace(payload.SourceProjectID) != "" {
		if payload.RunMode != 1 {
			return services.NonRetryableError{Err: fmt.Errorf("practical.audio.align.v1 replay entry requires run_mode=1")}
		}
		replayPayload, err := practicalreplay.PrepareGeneratePayload(payload, msg.Payload)
		if err != nil {
			return err
		}
		normalizedTasks, err := practicalreplay.ValidateSpecifyTasks(replayPayload.RunMode, replayPayload.SpecifyTasks)
		if err != nil {
			return err
		}
		replayPayload.SpecifyTasks = normalizedTasks
		return alignAndContinue(ctx, ch, replayPayload)
	}
	return alignAndContinue(ctx, ch, payload)
}

func handleRunModeFresh(ctx context.Context, ch *amqp.Channel, payload dto.PracticalAudioGeneratePayload) error {
	if err := validateFreshGeneratePayload(payload); err != nil {
		return err
	}
	if err := practicalreplay.PersistGeneratePayload(payload); err != nil {
		return err
	}
	return generateAndContinue(ctx, ch, payload)
}

func handleRunModeReplay(ctx context.Context, ch *amqp.Channel, payload dto.PracticalAudioGeneratePayload, rawPayload map[string]interface{}) error {
	replayPayload, err := practicalreplay.PrepareGeneratePayload(payload, rawPayload)
	if err != nil {
		return err
	}
	normalizedTasks, err := practicalreplay.ValidateSpecifyTasks(replayPayload.RunMode, replayPayload.SpecifyTasks)
	if err != nil {
		return err
	}
	replayPayload.SpecifyTasks = normalizedTasks
	if containsPracticalTask(replayPayload.SpecifyTasks, string(practicalreplay.PracticalStageGenerate)) && len(compactPositiveInts(replayPayload.BlockNums)) == 0 {
		return services.NonRetryableError{Err: fmt.Errorf("block_nums is required when specify_tasks includes generate")}
	}
	return generateAndContinue(ctx, ch, replayPayload)
}

func validateFreshGeneratePayload(payload dto.PracticalAudioGeneratePayload) error {
	if strings.TrimSpace(payload.ProjectID) == "" {
		return fmt.Errorf("project_id is required")
	}
	if _, err := practicalaudioservice.RequireLanguageForValidation(payload.Lang); err != nil {
		return err
	}
	if strings.TrimSpace(payload.ScriptFilename) == "" {
		return fmt.Errorf("script_filename is required")
	}
	if len(compactNonEmptyStrings(payload.BgImgFilenames)) == 0 {
		return fmt.Errorf("bg_img_filenames is required")
	}
	if payload.DesignType != 0 && normalizePracticalDesignType(payload.DesignType) != payload.DesignType {
		return fmt.Errorf("design_type must be 1 or 2")
	}
	if payload.TTSType != 0 && normalizePracticalTTSType(payload.TTSType) != payload.TTSType {
		return fmt.Errorf("tts_type must be 1")
	}
	return nil
}

func generateAndContinue(ctx context.Context, ch *amqp.Channel, payload dto.PracticalAudioGeneratePayload) error {
	_, err := practicalaudioservice.Generate(ctx, practicalaudioservice.GenerateInput{
		ProjectID:      payload.ProjectID,
		Language:       payload.Lang,
		ScriptFilename: payload.ScriptFilename,
		BlockNums:      compactPositiveInts(payload.BlockNums),
	})
	if err != nil {
		return err
	}
	return publishNextPracticalTaskFromGeneratePayload(ch, payload, string(practicalreplay.PracticalStageGenerate))
}

func alignAndContinue(ctx context.Context, ch *amqp.Channel, payload dto.PracticalAudioGeneratePayload) error {
	_, err := practicalaudioservice.Align(ctx, practicalaudioservice.AlignInput{
		ProjectID: payload.ProjectID,
		Language:  payload.Lang,
	})
	if err != nil {
		return err
	}
	return publishNextPracticalTaskFromGeneratePayload(ch, payload, string(practicalreplay.PracticalStageAlign))
}

func publishNextPracticalTaskFromGeneratePayload(ch *amqp.Channel, payload dto.PracticalAudioGeneratePayload, currentStage string) error {
	nextStage, ok, err := practicalreplay.NextPracticalStage(currentStage, payload.SpecifyTasks)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	taskType, err := practicalreplay.PracticalTaskTypeForStage(practicalreplay.PracticalStage(nextStage))
	if err != nil {
		return err
	}
	return publishPracticalAudioTask(ch, taskType, payload)
}

func publishPracticalAudioTask(ch *amqp.Channel, taskType string, payload dto.PracticalAudioGeneratePayload) error {
	return task.PublishTask(ch, taskType, buildPracticalAudioTaskPayload(payload))
}

func buildPracticalAudioTaskPayload(payload dto.PracticalAudioGeneratePayload) map[string]interface{} {
	out := map[string]interface{}{
		"content_type":    "practical",
		"project_id":      strings.TrimSpace(payload.ProjectID),
		"run_mode":        payload.RunMode,
		"tts_type":        normalizePracticalTTSType(payload.TTSType),
		"lang":            strings.TrimSpace(payload.Lang),
		"script_filename": strings.TrimSpace(payload.ScriptFilename),
	}
	if sourceProjectID := strings.TrimSpace(payload.SourceProjectID); sourceProjectID != "" {
		out["source_project_id"] = sourceProjectID
	}
	if tasks := compactNonEmptyStrings(payload.SpecifyTasks); len(tasks) > 0 && payload.RunMode == 1 {
		out["specify_tasks"] = tasks
	}
	if len(payload.BlockNums) > 0 {
		out["block_nums"] = compactPositiveInts(payload.BlockNums)
	}
	if backgrounds := compactNonEmptyStrings(payload.BgImgFilenames); len(backgrounds) > 0 {
		out["bg_img_filenames"] = backgrounds
	}
	if blockBackgrounds := compactNonEmptyStrings(payload.BlockBgImgFilenames); len(blockBackgrounds) > 0 {
		out["block_bg_img_filenames"] = blockBackgrounds
	}
	if resolution := strings.TrimSpace(payload.Resolution); resolution != "" {
		out["resolution"] = resolution
	}
	if aspectRatio := strings.TrimSpace(payload.AspectRatio); aspectRatio != "" {
		out["aspect_ratio"] = aspectRatio
	}
	if designType := normalizePracticalDesignType(payload.DesignType); designType > 0 {
		out["design_type"] = designType
	}
	return out
}

func decodePayload(raw map[string]interface{}) (dto.PracticalAudioGeneratePayload, error) {
	data, err := json.Marshal(raw)
	if err != nil {
		return dto.PracticalAudioGeneratePayload{}, err
	}
	var payload dto.PracticalAudioGeneratePayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return dto.PracticalAudioGeneratePayload{}, err
	}
	return payload, nil
}

func containsPracticalTask(values []string, target string) bool {
	target = strings.TrimSpace(target)
	for _, raw := range values {
		if strings.TrimSpace(raw) == target {
			return true
		}
	}
	return false
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

func normalizePracticalDesignType(value int) int {
	if value == 2 {
		return 2
	}
	return 1
}

func normalizePracticalTTSType(value int) int {
	if value == 1 {
		return 1
	}
	return 1
}
