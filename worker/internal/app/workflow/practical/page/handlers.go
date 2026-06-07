package practical_page

import (
	"context"
	"fmt"
	"strings"

	"worker/internal/app/task"
	practicalpipeline "worker/internal/app/workflow/practical/pipeline"
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
	if shouldInvalidate(string(practicalpipeline.StagePersist), payload.StartFrom) {
		if err := practicalpipeline.InvalidateOutputs(payload.ProjectID, payload.StartFrom); err != nil {
			return err
		}
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
	payload, err = practicalpipeline.ResolvePayload(payload)
	if err != nil {
		return dto.PracticalAudioGeneratePayload{}, err
	}
	return payload, nil
}

func publishNextPracticalTaskFromPersistPayload(ch *amqp.Channel, payload dto.PracticalAudioGeneratePayload) error {
	nextStage, ok, err := practicalpipeline.NextStage(string(practicalpipeline.StagePersist), payload.StopAt)
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
	return task.PublishTask(ch, taskType, buildPracticalPersistTaskPayload(payload))
}

func buildPracticalPersistTaskPayload(payload dto.PracticalAudioGeneratePayload) map[string]interface{} {
	out := map[string]interface{}{
		"content_type": "practical",
		"project_id":   strings.TrimSpace(payload.ProjectID),
		"run_mode":     payload.RunMode,
		"tts_type":     normalizePracticalTTSType(payload.TTSType),
		"start_from":   strings.TrimSpace(payload.StartFrom),
	}
	if stopAt := strings.TrimSpace(payload.StopAt); stopAt != "" {
		out["stop_at"] = stopAt
	}
	if chapterNums := compactPositiveInts(payload.ChapterNums); len(chapterNums) > 0 {
		out["chapter_nums"] = chapterNums
	}
	if blockNums := compactPositiveInts(payload.BlockNums); len(blockNums) > 0 {
		out["block_nums"] = blockNums
	}
	if lang := strings.TrimSpace(payload.Lang); lang != "" {
		out["lang"] = lang
	}
	if scriptFile := strings.TrimSpace(payload.ScriptFilename); scriptFile != "" {
		out["script_filename"] = scriptFile
	}
	return out
}

func decodePayload(raw map[string]interface{}) (dto.PracticalAudioGeneratePayload, error) {
	return practicalpipeline.DecodePayload(raw)
}

func normalizePracticalTTSType(value int) int {
	if value == 1 {
		return 1
	}
	return 1
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

func shouldInvalidate(currentStage string, startFrom string) bool {
	return strings.TrimSpace(currentStage) == strings.TrimSpace(startFrom)
}
