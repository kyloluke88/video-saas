package podcast_compose_service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	conf "worker/pkg/config"
	services "worker/services"
	dto "worker/services/podcast/model"
	ffmpegpodcast "worker/services/podcast/render"
)

type ComposeInput struct {
	ProjectID      string
	Language       string
	BgImgFilenames []string
	Resolution     string
	DesignStyle    int
}

type ComposeResult struct {
	FinalVideoPath string
}

type ComposeRenderResult struct {
	BaseVideoPath string
}

func Render(ctx context.Context, input ComposeInput) (ComposeRenderResult, error) {
	if strings.TrimSpace(input.ProjectID) == "" {
		return ComposeRenderResult{}, fmt.Errorf("project_id is required")
	}
	if _, err := requirePodcastLanguage(input.Language); err != nil {
		return ComposeRenderResult{}, err
	}
	paths, err := prepareComposeArtifacts(input)
	if err != nil {
		return ComposeRenderResult{}, err
	}
	log.Printf("📦 podcast compose assets project_id=%s dialogue=%s script=%s background=%s", input.ProjectID, paths.DialoguePath, paths.ScriptPath, paths.BackgroundPath)
	if err := ffmpegpodcast.ComposeBaseVideoContext(ctx, ffmpegpodcast.ComposeInput{
		BackgroundImagePath: paths.BackgroundPath,
		DialogueAudioPath:   paths.DialoguePath,
		Resolution:          paths.Resolution,
		DesignStyle:         input.DesignStyle,
		OutputPath:          paths.FinalVideoPath,
	}); err != nil {
		return ComposeRenderResult{}, err
	}
	return ComposeRenderResult{BaseVideoPath: paths.BaseVideoPath}, nil
}

func Finalize(ctx context.Context, input ComposeInput) (ComposeResult, error) {
	if strings.TrimSpace(input.ProjectID) == "" {
		return ComposeResult{}, fmt.Errorf("project_id is required")
	}
	language, err := requirePodcastLanguage(input.Language)
	if err != nil {
		return ComposeResult{}, err
	}
	paths, err := prepareComposeArtifacts(input)
	if err != nil {
		return ComposeResult{}, err
	}

	var script dto.PodcastScript
	if err := readJSON(paths.ScriptPath, &script); err != nil {
		return ComposeResult{}, err
	}
	if err := validateScriptLanguage(script.Language, language); err != nil {
		return ComposeResult{}, err
	}
	script.Language = language
	script.RefreshSegmentsFromBlocks()
	if err := ffmpegpodcast.FinalizeComposedVideoContext(ctx, ffmpegpodcast.ComposeInput{
		DialogueAudioPath: paths.DialoguePath,
		Script:            &script,
		Resolution:        paths.Resolution,
		DesignStyle:       input.DesignStyle,
		OutputPath:        paths.FinalVideoPath,
	}); err != nil {
		return ComposeResult{}, err
	}
	if err := cleanupPodcastIntermediates(paths.ProjectDir); err != nil {
		log.Printf("⚠️ podcast compose cleanup warning project_id=%s err=%v", input.ProjectID, err)
	}
	return ComposeResult{FinalVideoPath: paths.FinalVideoPath}, nil
}

type composeArtifacts struct {
	ProjectDir     string
	DialoguePath   string
	ScriptPath     string
	BackgroundPath string
	BaseVideoPath  string
	FinalVideoPath string
	Resolution     string
}

func prepareComposeArtifacts(input ComposeInput) (composeArtifacts, error) {
	backgroundPath, err := backgroundImagePathForRequest(input.BgImgFilenames)
	if err != nil {
		return composeArtifacts{}, err
	}

	projectDir := projectDirFor(input.ProjectID)
	artifacts := composeArtifacts{
		ProjectDir:     projectDir,
		DialoguePath:   filepath.Join(projectDir, "dialogue.mp3"),
		ScriptPath:     filepath.Join(projectDir, "script_aligned.json"),
		BackgroundPath: backgroundPath,
		BaseVideoPath:  filepath.Join(projectDir, "podcast_base.mp4"),
		FinalVideoPath: filepath.Join(projectDir, "podcast_final.mp4"),
		Resolution:     defaultPodcastResolution(input.Resolution),
	}

	if _, err := os.Stat(artifacts.DialoguePath); err != nil {
		return composeArtifacts{}, fmt.Errorf("dialogue audio missing: %s", artifacts.DialoguePath)
	}
	if _, err := os.Stat(artifacts.ScriptPath); err != nil {
		return composeArtifacts{}, fmt.Errorf("aligned script missing: %s", artifacts.ScriptPath)
	}
	if _, err := os.Stat(artifacts.BackgroundPath); err != nil {
		return composeArtifacts{}, fmt.Errorf("background image missing: %s", artifacts.BackgroundPath)
	}
	return artifacts, nil
}

func projectDirFor(projectID string) string {
	return filepath.Join(conf.Get[string]("worker.ffmpeg_work_dir"), "projects", projectID)
}

func backgroundImagePathFor(filename string) string {
	return filepath.Join(conf.Get[string]("worker.worker_assets_dir"), "podcast", "bg-images", filepath.Base(strings.TrimSpace(filename)))
}

func backgroundImagePathForRequest(many []string) (string, error) {
	filenames := compactBackgroundNames(many)
	if len(filenames) == 0 {
		return "", fmt.Errorf("bg_img_filenames is required")
	}
	// Static background mode: only the first image is used for all design styles.
	return backgroundImagePathFor(filenames[0]), nil
}

func compactBackgroundNames(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func readJSON(path string, out interface{}) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, out)
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
