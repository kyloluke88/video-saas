package practical_compose

import (
	"context"
	"fmt"
	"strings"

	"worker/internal/app/task"
	practicalpipeline "worker/internal/app/workflow/practical/pipeline"
	services "worker/services"
	practicalcomposeservice "worker/services/practical/compose"
	dto "worker/services/practical/model"

	amqp "github.com/rabbitmq/amqp091-go"
)

func HandleComposeRender(ctx context.Context, ch *amqp.Channel, msg task.VideoTaskMessage) error {
	payload, err := resolveComposePayload(msg.Payload)
	if err != nil {
		return err
	}
	if shouldInvalidate(string(practicalpipeline.StageRender), payload.StartFrom) {
		if err := practicalpipeline.InvalidateOutputs(payload.ProjectID, payload.TTSType, payload.StartFrom); err != nil {
			return err
		}
	}

	_, err = practicalcomposeservice.Render(ctx, composeInputFromPayload(payload))
	if err != nil {
		return err
	}
	return publishNextPracticalTaskFromComposePayload(ch, payload, string(practicalpipeline.StageRender))
}

func resolveComposePayload(raw map[string]interface{}) (dto.PracticalAudioGeneratePayload, error) {
	payload, err := decodePayload(raw)
	if err != nil {
		return dto.PracticalAudioGeneratePayload{}, err
	}
	if payload.RunMode != 0 && payload.RunMode != 1 {
		return dto.PracticalAudioGeneratePayload{}, services.NonRetryableError{Err: fmt.Errorf("practical.compose only supports run_mode 0 or 1")}
	}
	payload, err = practicalpipeline.ResolvePayload(payload)
	if err != nil {
		return dto.PracticalAudioGeneratePayload{}, err
	}
	if strings.TrimSpace(payload.ProjectID) == "" {
		return dto.PracticalAudioGeneratePayload{}, fmt.Errorf("project_id is required")
	}
	if strings.TrimSpace(payload.Lang) == "" {
		return dto.PracticalAudioGeneratePayload{}, fmt.Errorf("lang is required")
	}
	return payload, nil
}

func composeInputFromPayload(payload dto.PracticalAudioGeneratePayload) practicalcomposeservice.ComposeInput {
	return practicalcomposeservice.ComposeInput{
		ProjectID:  payload.ProjectID,
		Language:   payload.Lang,
		Resolution: payload.Resolution,
		DesignType: payload.DesignType,
	}
}

func publishNextPracticalTaskFromComposePayload(ch *amqp.Channel, payload dto.PracticalAudioGeneratePayload, currentStage string) error {
	nextStage, ok, err := practicalpipeline.NextStage(normalizePracticalTTSType(payload.TTSType), currentStage, payload.StopAt)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	taskType, err := practicalpipeline.TaskTypeForStage(normalizePracticalTTSType(payload.TTSType), practicalpipeline.Stage(nextStage))
	if err != nil {
		return err
	}
	return task.PublishTask(ch, taskType, buildPracticalComposeTaskPayload(payload))
}

func buildPracticalComposeTaskPayload(payload dto.PracticalAudioGeneratePayload) map[string]interface{} {
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

func shouldInvalidate(currentStage string, startFrom string) bool {
	return strings.TrimSpace(currentStage) == strings.TrimSpace(startFrom)
}
