package practical_audio

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"worker/internal/app/task"
	practicalpipeline "worker/internal/app/workflow/practical/pipeline"
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
	if payload.RunMode != 0 && payload.RunMode != 1 {
		return services.NonRetryableError{Err: fmt.Errorf("practical.audio.generate.v1 only supports run_mode 0 or 1")}
	}
	payload, err = practicalpipeline.ResolvePayload(payload)
	if err != nil {
		return err
	}
	if err := validateFreshGeneratePayload(payload); err != nil {
		return err
	}
	if shouldInvalidate(string(practicalpipeline.StageGenerate), payload.StartFrom) {
		if err := practicalpipeline.InvalidateOutputs(payload.ProjectID, payload.StartFrom); err != nil {
			return err
		}
	}
	if err := practicalpipeline.PersistPayload(payload); err != nil {
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
		return services.NonRetryableError{Err: fmt.Errorf("practical.audio.align.v1 only supports run_mode 0 or 1")}
	}
	payload, err = practicalpipeline.ResolvePayload(payload)
	if err != nil {
		return err
	}
	if shouldInvalidate(string(practicalpipeline.StageAlign), payload.StartFrom) {
		if err := practicalpipeline.InvalidateOutputs(payload.ProjectID, payload.StartFrom); err != nil {
			return err
		}
	}
	if err := practicalpipeline.PersistPayload(payload); err != nil {
		return err
	}
	return alignAndContinue(ctx, ch, payload)
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
		ChapterNums:    compactPositiveInts(payload.ChapterNums),
	})
	if err != nil {
		return err
	}
	return publishNextPracticalTaskFromGeneratePayload(ch, payload, string(practicalpipeline.StageGenerate))
}

func alignAndContinue(ctx context.Context, ch *amqp.Channel, payload dto.PracticalAudioGeneratePayload) error {
	_, err := practicalaudioservice.Align(ctx, practicalaudioservice.AlignInput{
		ProjectID:   payload.ProjectID,
		Language:    payload.Lang,
		BlockNums:   compactPositiveInts(payload.BlockNums),
		ChapterNums: compactPositiveInts(payload.ChapterNums),
	})
	if err != nil {
		return err
	}
	return publishNextPracticalTaskFromGeneratePayload(ch, payload, string(practicalpipeline.StageAlign))
}

func publishNextPracticalTaskFromGeneratePayload(ch *amqp.Channel, payload dto.PracticalAudioGeneratePayload, currentStage string) error {
	nextStage, ok, err := practicalpipeline.NextStage(currentStage, payload.StopAt)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	taskType, err := practicalpipeline.TaskTypeForStage(practicalpipeline.Stage(nextStage))
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
		"start_from":      strings.TrimSpace(payload.StartFrom),
	}
	if stopAt := strings.TrimSpace(payload.StopAt); stopAt != "" {
		out["stop_at"] = stopAt
	}
	if len(payload.BlockNums) > 0 {
		out["block_nums"] = compactPositiveInts(payload.BlockNums)
	}
	if chapterNums := compactPositiveInts(payload.ChapterNums); len(chapterNums) > 0 {
		out["chapter_nums"] = chapterNums
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

func shouldInvalidate(currentStage string, startFrom string) bool {
	return strings.TrimSpace(currentStage) == strings.TrimSpace(startFrom)
}
