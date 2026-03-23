package podcast_audio

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"worker/internal/dto"
	"worker/internal/pipeline"
	conf "worker/pkg/config"
	podcastaudioservice "worker/services/podcast_audio_service"

	amqp "github.com/rabbitmq/amqp091-go"
)

func HandleGenerate(ch *amqp.Channel, task dto.VideoTaskMessage) error {
	payload, err := decodePayload(task.Payload)
	if err != nil {
		return err
	}
	switch normalizePodcastRunMode(payload.RunMode) {
	case 1:
		return handleRunModeReplay(ch, payload.ProjectID)
	case 2:
		return handleRunModeComposeOnly(ch, payload)
	default:
		return handleRunModeFresh(ch, task.Payload, payload)
	}
}

func validPodcastLanguage(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "zh", "ja":
		return true
	default:
		return false
	}
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

func persistRequestPayload(projectID string, payload map[string]interface{}) error {
	projectDir := filepath.Join(conf.Get[string]("worker.ffmpeg_work_dir"), "projects", projectID)
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(projectDir, "request_payload.json"), data, 0o644)
}

func loadPersistedRequestPayload(projectID string) (map[string]interface{}, error) {
	projectDir := filepath.Join(conf.Get[string]("worker.ffmpeg_work_dir"), "projects", projectID)
	raw, err := os.ReadFile(filepath.Join(projectDir, "request_payload.json"))
	if err != nil {
		return nil, fmt.Errorf("project request payload not found for %s: %w", projectID, err)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("project request payload invalid for %s: %w", projectID, err)
	}
	if savedProjectID := strings.TrimSpace(fmt.Sprint(payload["project_id"])); savedProjectID != "" && savedProjectID != projectID {
		return nil, fmt.Errorf("project request payload mismatch requested=%s payload=%s", projectID, savedProjectID)
	}
	return payload, nil
}

func handleRunModeFresh(ch *amqp.Channel, rawPayload map[string]interface{}, payload dto.PodcastAudioGeneratePayload) error {
	if err := validateFreshGeneratePayload(payload); err != nil {
		return err
	}
	if err := persistRequestPayload(payload.ProjectID, rawPayload); err != nil {
		return err
	}
	return generateAndPublishCompose(ch, payload)
}

func handleRunModeReplay(ch *amqp.Channel, projectID string) error {
	savedPayload, err := loadPersistedGeneratePayload(projectID)
	if err != nil {
		return err
	}
	log.Printf("♻️ podcast run_mode=1 replay project_id=%s", projectID)
	savedPayload.RunMode = 0
	return generateAndPublishCompose(ch, savedPayload)
}

func handleRunModeComposeOnly(ch *amqp.Channel, payload dto.PodcastAudioGeneratePayload) error {
	savedPayload, err := loadPersistedGeneratePayload(payload.ProjectID)
	if err != nil {
		return err
	}
	composePayload, err := buildComposePayloadForRunMode2(savedPayload, payload)
	if err != nil {
		return err
	}
	log.Printf("🎬 podcast run_mode=2 compose-only project_id=%s background=%s design_style=%d resolution=%s",
		composePayload.ProjectID, composePayload.BgImgFilename, composePayload.DesignStyle, composePayload.Resolution)
	return publishComposeTask(ch, composePayload)
}

func loadPersistedGeneratePayload(projectID string) (dto.PodcastAudioGeneratePayload, error) {
	if strings.TrimSpace(projectID) == "" {
		return dto.PodcastAudioGeneratePayload{}, fmt.Errorf("project_id is required")
	}
	rawPayload, err := loadPersistedRequestPayload(projectID)
	if err != nil {
		return dto.PodcastAudioGeneratePayload{}, err
	}
	payload, err := decodePayload(rawPayload)
	if err != nil {
		return dto.PodcastAudioGeneratePayload{}, err
	}
	if strings.TrimSpace(payload.ProjectID) == "" {
		payload.ProjectID = strings.TrimSpace(projectID)
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
	if strings.TrimSpace(payload.ScriptFilename) == "" {
		return fmt.Errorf("script_filename is required")
	}
	if strings.TrimSpace(payload.BgImgFilename) == "" {
		return fmt.Errorf("bg_img_filename is required")
	}
	return nil
}

func generateAndPublishCompose(ch *amqp.Channel, payload dto.PodcastAudioGeneratePayload) error {
	result, err := podcastaudioservice.Generate(podcastaudioservice.GenerateInput{
		ProjectID:      payload.ProjectID,
		Language:       payload.Lang,
		ScriptFilename: payload.ScriptFilename,
	})
	if err != nil {
		return err
	}

	log.Printf("🎧 podcast audio generated project_id=%s audio=%s script=%s", payload.ProjectID, result.DialogueAudioPath, result.AlignedScriptPath)
	return publishComposeTask(ch, dto.PodcastComposePayload{
		ProjectID:      payload.ProjectID,
		Lang:           payload.Lang,
		Title:          payload.Title,
		BgImgFilename:  payload.BgImgFilename,
		TargetPlatform: payload.TargetPlatform,
		AspectRatio:    payload.AspectRatio,
		Resolution:     payload.Resolution,
		DesignStyle:    payload.DesignStyle,
	})
}

func publishComposeTask(ch *amqp.Channel, payload dto.PodcastComposePayload) error {
	return pipeline.PublishTask(ch, "podcast.compose.v1", map[string]interface{}{
		"project_id":      payload.ProjectID,
		"lang":            payload.Lang,
		"title":           payload.Title,
		"bg_img_filename": payload.BgImgFilename,
		"target_platform": payload.TargetPlatform,
		"aspect_ratio":    payload.AspectRatio,
		"resolution":      payload.Resolution,
		"design_style":    payload.DesignStyle,
	})
}
