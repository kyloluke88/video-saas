package practical_page

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"worker/internal/app/task"
	practicalreplay "worker/internal/app/workflow/practical/replay"
	services "worker/services"
	dto "worker/services/practical/model"
	practicalpageservice "worker/services/practical/page"

	amqp "github.com/rabbitmq/amqp091-go"
)

func HandlePersist(_ context.Context, ch *amqp.Channel, msg task.VideoTaskMessage) error {
	payload, err := resolvePersistPayload(msg.Payload)
	if err != nil {
		return err
	}

	if strings.TrimSpace(payload.ProjectID) == "" {
		return fmt.Errorf("project_id is required")
	}

	if _, err := practicalpageservice.Persist(practicalpageservice.PersistInput{
		ProjectID: strings.TrimSpace(payload.ProjectID),
	}); err != nil {
		return err
	}

	return publishNextPracticalTaskFromPersistPayload(ch, payload)
}

func resolvePersistPayload(raw map[string]interface{}) (dto.PracticalAudioGeneratePayload, error) {
	payload, err := decodePayload(raw)
	if err != nil {
		return dto.PracticalAudioGeneratePayload{}, err
	}
	if payload.RunMode != 0 && payload.RunMode != 1 {
		return dto.PracticalAudioGeneratePayload{}, services.NonRetryableError{Err: fmt.Errorf("practical.page.persist.v1 only supports run_mode 0 or 1")}
	}
	if payload.RunMode == 1 || strings.TrimSpace(payload.SourceProjectID) != "" {
		if payload.RunMode != 1 {
			return dto.PracticalAudioGeneratePayload{}, services.NonRetryableError{Err: fmt.Errorf("persist replay entry requires run_mode=1")}
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
		return replayPayload, nil
	}
	return payload, nil
}

func publishNextPracticalTaskFromPersistPayload(ch *amqp.Channel, payload dto.PracticalAudioGeneratePayload) error {
	nextStage, ok, err := practicalreplay.NextPracticalStage(string(practicalreplay.PracticalStagePersist), payload.SpecifyTasks)
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
	return task.PublishTask(ch, taskType, buildPracticalPersistTaskPayload(payload))
}

func buildPracticalPersistTaskPayload(payload dto.PracticalAudioGeneratePayload) map[string]interface{} {
	out := map[string]interface{}{
		"content_type": "practical",
		"project_id":   strings.TrimSpace(payload.ProjectID),
		"run_mode":     payload.RunMode,
		"tts_type":     normalizePracticalTTSType(payload.TTSType),
	}
	if sourceProjectID := strings.TrimSpace(payload.SourceProjectID); sourceProjectID != "" {
		out["source_project_id"] = sourceProjectID
	}
	if tasks := compactNonEmptyStrings(payload.SpecifyTasks); len(tasks) > 0 && payload.RunMode == 1 {
		out["specify_tasks"] = tasks
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

func normalizePracticalTTSType(value int) int {
	if value == 1 {
		return 1
	}
	return 1
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
