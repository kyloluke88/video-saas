package podcast_compose_service

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"worker/internal/dto"
	conf "worker/pkg/config"
	services "worker/services"
	ffmpegpodcast "worker/services/ffmpeg_service/podcast"
)

type ComposeInput struct {
	ProjectID     string
	Language      string
	BgImgFilename string
	Resolution    string
	DesignStyle   int
}

type ComposeResult struct {
	FinalVideoPath string
}

func Compose(input ComposeInput) (ComposeResult, error) {
	if strings.TrimSpace(input.ProjectID) == "" {
		return ComposeResult{}, fmt.Errorf("project_id is required")
	}
	language, err := requirePodcastLanguage(input.Language)
	if err != nil {
		return ComposeResult{}, err
	}
	if strings.TrimSpace(input.BgImgFilename) == "" {
		return ComposeResult{}, fmt.Errorf("bg_img_filename is required")
	}
	log.Printf("🎬 podcast compose start project_id=%s resolution=%s design_style=%d",
		input.ProjectID, defaultPodcastResolution(input.Resolution), input.DesignStyle)

	projectDir := projectDirFor(input.ProjectID)
	dialoguePath := filepath.Join(projectDir, "dialogue.mp3")
	scriptPath := filepath.Join(projectDir, "script_aligned.json")
	backgroundPath := backgroundImagePathFor(input.BgImgFilename)

	if _, err := os.Stat(dialoguePath); err != nil {
		return ComposeResult{}, fmt.Errorf("dialogue audio missing: %s", dialoguePath)
	}
	if _, err := os.Stat(backgroundPath); err != nil {
		return ComposeResult{}, fmt.Errorf("background image missing: %s", backgroundPath)
	}
	log.Printf("📦 podcast compose assets project_id=%s dialogue=%s script=%s background=%s", input.ProjectID, dialoguePath, scriptPath, backgroundPath)

	var script dto.PodcastScript
	if err := readJSON(scriptPath, &script); err != nil {
		return ComposeResult{}, err
	}
	if err := validateScriptLanguage(script.Language, language); err != nil {
		return ComposeResult{}, err
	}
	script.Language = language
	script.RefreshSegmentsFromBlocks()
	log.Printf("📝 podcast compose script project_id=%s segments=%d", input.ProjectID, len(script.Segments))

	finalPath := filepath.Join(projectDir, "podcast_final.mp4")
	if err := ffmpegpodcast.ComposeVideo(ffmpegpodcast.ComposeInput{
		BackgroundImagePath: backgroundPath,
		DialogueAudioPath:   dialoguePath,
		Script:              &script,
		Resolution:          defaultPodcastResolution(input.Resolution),
		DesignStyle:         input.DesignStyle,
		OutputPath:          finalPath,
	}); err != nil {
		return ComposeResult{}, err
	}
	if err := cleanupPodcastIntermediates(projectDir); err != nil {
		log.Printf("⚠️ podcast compose cleanup warning project_id=%s err=%v", input.ProjectID, err)
	}
	log.Printf("✅ podcast compose output project_id=%s final=%s", input.ProjectID, finalPath)
	return ComposeResult{FinalVideoPath: finalPath}, nil
}

func projectDirFor(projectID string) string {
	return filepath.Join(conf.Get[string]("worker.ffmpeg_work_dir"), "projects", projectID)
}

func backgroundImagePathFor(filename string) string {
	return filepath.Join(conf.Get[string]("worker.worker_assets_dir"), "podcast", "bg-images", filepath.Base(strings.TrimSpace(filename)))
}

func readJSON(path string, out interface{}) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, out)
}

func defaultString(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return v
}

func defaultPodcastResolution(value string) string {
	if strings.TrimSpace(value) != "" {
		return value
	}
	if strings.TrimSpace(conf.Get[string]("worker.podcast_mode", "debug")) == "production" {
		return "1080p"
	}
	return "480p"
}

func requirePodcastLanguage(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "zh":
		return "zh", nil
	case "ja":
		return "ja", nil
	default:
		return "", fmt.Errorf("lang must be zh or ja")
	}
}

func validateScriptLanguage(scriptLanguage, payloadLanguage string) error {
	scriptLang, err := requirePodcastLanguage(scriptLanguage)
	if err != nil {
		return services.NonRetryableError{Err: fmt.Errorf("script language mismatch: script=%q payload=%q", strings.TrimSpace(scriptLanguage), payloadLanguage)}
	}
	if scriptLang != payloadLanguage {
		return services.NonRetryableError{Err: fmt.Errorf("script language mismatch: script=%q payload=%q", scriptLang, payloadLanguage)}
	}
	return nil
}
