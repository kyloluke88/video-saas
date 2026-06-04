package practical_image

import (
	"context"
	"fmt"
	"strings"

	"worker/internal/app/task"
	practicalreplay "worker/internal/app/workflow/practical/replay"
	services "worker/services"
	practicalimageservice "worker/services/practical/image"
	dto "worker/services/practical/model"

	amqp "github.com/rabbitmq/amqp091-go"
)

func HandleGenerate(ctx context.Context, ch *amqp.Channel, msg task.VideoTaskMessage) error {
	payload, err := resolveImagePayload(msg.Payload)
	if err != nil {
		return err
	}

	_, err = practicalimageservice.Generate(ctx, practicalimageservice.GenerateInput{
		ProjectID:   payload.ProjectID,
		Language:    payload.Lang,
		Resolution:  payload.Resolution,
		AspectRatio: payload.AspectRatio,
		BlockNums:   compactPositiveInts(payload.BlockNums),
	})
	if err != nil {
		return err
	}
	return publishNextPracticalTaskFromImagePayload(ch, payload, string(practicalreplay.PracticalStageImages))
}

func resolveImagePayload(raw map[string]interface{}) (dto.PracticalAudioGeneratePayload, error) {
	payload, err := practicalreplay.DecodeGeneratePayload(raw)
	if err != nil {
		return dto.PracticalAudioGeneratePayload{}, err
	}
	if payload.RunMode != 0 && payload.RunMode != 1 {
		return dto.PracticalAudioGeneratePayload{}, services.NonRetryableError{Err: fmt.Errorf("practical.image.generate.v1 only supports run_mode 0 or 1")}
	}
	if payload.RunMode == 1 || strings.TrimSpace(payload.SourceProjectID) != "" {
		if payload.RunMode != 1 {
			return dto.PracticalAudioGeneratePayload{}, services.NonRetryableError{Err: fmt.Errorf("image replay entry requires run_mode=1")}
		}
		replayPayload, err := practicalreplay.PrepareGeneratePayload(payload, raw)
		if err != nil {
			return dto.PracticalAudioGeneratePayload{}, err
		}
		normalizedTasks, err := practicalreplay.ValidateSpecifyTasks(replayPayload.RunMode, replayPayload.SpecifyTasks)
		if err != nil {
			return dto.PracticalAudioGeneratePayload{}, err
		}
		replayPayload.SpecifyTasks = normalizedTasks
		payload = replayPayload
	}
	if strings.TrimSpace(payload.ProjectID) == "" {
		return dto.PracticalAudioGeneratePayload{}, fmt.Errorf("project_id is required")
	}
	if strings.TrimSpace(payload.Lang) == "" {
		return dto.PracticalAudioGeneratePayload{}, fmt.Errorf("lang is required")
	}
	return payload, nil
}

func publishNextPracticalTaskFromImagePayload(ch *amqp.Channel, payload dto.PracticalAudioGeneratePayload, currentStage string) error {
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
	return task.PublishTask(ch, taskType, buildPracticalImageTaskPayload(payload))
}

func buildPracticalImageTaskPayload(payload dto.PracticalAudioGeneratePayload) map[string]interface{} {
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
	if blocks := compactPositiveInts(payload.BlockNums); len(blocks) > 0 {
		out["block_nums"] = blocks
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

func compactPositiveInts(values []int) []int {
	seen := make(map[int]struct{}, len(values))
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
