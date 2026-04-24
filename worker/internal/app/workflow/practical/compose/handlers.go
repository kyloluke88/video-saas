package practical_compose

import (
	"context"
	"fmt"
	"strings"

	"worker/internal/app/task"
	practicalreplay "worker/internal/app/workflow/practical/replay"
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

	_, err = practicalcomposeservice.Render(ctx, composeInputFromPayload(payload))
	if err != nil {
		return err
	}
	return publishNextPracticalTaskFromComposePayload(ch, payload, string(practicalreplay.PracticalStageRender))
}

func HandleComposeFinalize(ctx context.Context, ch *amqp.Channel, msg task.VideoTaskMessage) error {
	payload, err := resolveComposePayload(msg.Payload)
	if err != nil {
		return err
	}
	_, err = practicalcomposeservice.Finalize(ctx, composeInputFromPayload(payload))
	if err != nil {
		return err
	}
	return publishNextPracticalTaskFromComposePayload(ch, payload, string(practicalreplay.PracticalStageFinalize))
}

func resolveComposePayload(raw map[string]interface{}) (dto.PracticalAudioGeneratePayload, error) {
	payload, err := decodePayload(raw)
	if err != nil {
		return dto.PracticalAudioGeneratePayload{}, err
	}
	if payload.RunMode != 0 && payload.RunMode != 1 {
		return dto.PracticalAudioGeneratePayload{}, services.NonRetryableError{Err: fmt.Errorf("practical.compose only supports run_mode 0 or 1")}
	}
	if payload.RunMode == 1 || strings.TrimSpace(payload.SourceProjectID) != "" {
		if payload.RunMode != 1 {
			return dto.PracticalAudioGeneratePayload{}, services.NonRetryableError{Err: fmt.Errorf("compose replay entry requires run_mode=1")}
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
	if len(payload.BgImgFilenames) == 0 {
		return dto.PracticalAudioGeneratePayload{}, fmt.Errorf("bg_img_filenames is required")
	}
	return payload, nil
}

func composeInputFromPayload(payload dto.PracticalAudioGeneratePayload) practicalcomposeservice.ComposeInput {
	return practicalcomposeservice.ComposeInput{
		ProjectID:           payload.ProjectID,
		Language:            payload.Lang,
		BgImgFilenames:      payload.BgImgFilenames,
		BlockBgImgFilenames: payload.BlockBgImgFilenames,
		Resolution:          payload.Resolution,
		DesignType:          payload.DesignType,
	}
}

func publishNextPracticalTaskFromComposePayload(ch *amqp.Channel, payload dto.PracticalAudioGeneratePayload, currentStage string) error {
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
