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
	if strings.TrimSpace(payload.ProjectID) == "" {
		return fmt.Errorf("project_id is required")
	}
	if strings.TrimSpace(payload.ScriptFilename) == "" {
		return fmt.Errorf("script_filename is required")
	}
	if strings.TrimSpace(payload.BgImgFilename) == "" {
		return fmt.Errorf("bg_img_filename is required")
	}
	if err := persistRequestPayload(payload.ProjectID, task.Payload); err != nil {
		return err
	}

	result, err := podcastaudioservice.Generate(podcastaudioservice.GenerateInput{
		ProjectID:       payload.ProjectID,
		ScriptFilename:  payload.ScriptFilename,
		MaleVoiceType:   payload.MaleVoiceType,
		FemaleVoiceType: payload.FemaleVoiceType,
	})
	if err != nil {
		return err
	}

	log.Printf("🎧 podcast audio generated project_id=%s audio=%s script=%s", payload.ProjectID, result.DialogueAudioPath, result.AlignedScriptPath)
	return pipeline.PublishTask(ch, "podcast.compose.v1", map[string]interface{}{
		"project_id":      payload.ProjectID,
		"title":           payload.Title,
		"bg_img_filename": payload.BgImgFilename,
		"target_platform": payload.TargetPlatform,
		"aspect_ratio":    payload.AspectRatio,
		"resolution":      payload.Resolution,
		"design_style":    payload.DesignStyle,
	})
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
