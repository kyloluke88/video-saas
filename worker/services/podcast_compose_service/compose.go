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
	ffmpegpodcast "worker/services/ffmpeg_service/podcast"
)

type ComposeInput struct {
	ProjectID     string
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
	log.Printf("✅ podcast compose output project_id=%s final=%s", input.ProjectID, finalPath)
	if err := cleanupProjectArtifacts(projectDir); err != nil {
		return ComposeResult{}, err
	}

	return ComposeResult{FinalVideoPath: finalPath}, nil
}

func projectDirFor(projectID string) string {
	return filepath.Join(conf.Get[string]("worker.ffmpeg_work_dir"), "projects", projectID)
}

func backgroundImagePathFor(filename string) string {
	return filepath.Join(conf.Get[string]("worker.ffmpeg_work_dir"), "podcast", "bg-images", filepath.Base(strings.TrimSpace(filename)))
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

func cleanupProjectArtifacts(projectDir string) error {
	keep := map[string]struct{}{
		"podcast_final.mp4":   {},
		"dialogue.mp3":        {},
		"script_aligned.json": {},
	}
	if conf.Get[bool]("worker.podcast_keep_ass", false) {
		keep["podcast_subtitles.ass"] = struct{}{}
	}

	entries, err := os.ReadDir(projectDir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		name := entry.Name()
		if _, ok := keep[name]; ok {
			continue
		}
		if err := os.RemoveAll(filepath.Join(projectDir, name)); err != nil {
			return fmt.Errorf("cleanup %s failed: %w", name, err)
		}
	}
	return nil
}
