package podcast

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	conf "worker/pkg/config"
	"worker/services/ffmpeg_service/common"
)

func ComposeVideo(input ComposeInput) error {
	if strings.TrimSpace(input.BackgroundImagePath) == "" {
		return fmt.Errorf("background image is required")
	}
	if strings.TrimSpace(input.DialogueAudioPath) == "" {
		return fmt.Errorf("dialogue audio is required")
	}
	if strings.TrimSpace(input.OutputPath) == "" {
		return fmt.Errorf("output path is required")
	}

	projectDir := filepath.Dir(input.OutputPath)
	scale := common.ResolutionToScale(input.Resolution)
	wave := waveformPresetFor(input.DesignStyle, input.Resolution)
	x264Preset := podcastX264Preset()

	baseOutput := filepath.Join(projectDir, "podcast_base.mp4")
	bgFilter := fmt.Sprintf("[0:v]scale=%s", scale)
	if strings.TrimSpace(wave.BackgroundFilter) != "" {
		bgFilter += "," + wave.BackgroundFilter
	}
	bgFilter += "[bg]"
	complexFilter := fmt.Sprintf("%s;%s;[bg][sw]overlay=%s[v]", bgFilter, wave.AudioGraph, wave.Overlay)
	if err := common.RunFFmpeg(
		"-y",
		"-loop", "1",
		"-i", input.BackgroundImagePath,
		"-i", input.DialogueAudioPath,
		"-filter_complex", complexFilter,
		"-map", "[v]",
		"-map", "1:a:0",
		"-c:v", "libx264",
		"-preset", x264Preset,
		"-pix_fmt", "yuv420p",
		"-c:a", "aac",
		"-shortest",
		baseOutput,
	); err != nil {
		return err
	}

	if input.Script == nil {
		return os.Rename(baseOutput, input.OutputPath)
	}

	assPath, err := WritePodcastASS(*input.Script, projectDir, input.Resolution, input.DesignStyle)
	if err != nil {
		return err
	}
	if strings.TrimSpace(assPath) == "" {
		return os.Rename(baseOutput, input.OutputPath)
	}

	filter := fmt.Sprintf("subtitles=%s:fontsdir=%s", escapeFFmpegPath(assPath), escapeFFmpegPath(podcastFontsDir()))
	if err := common.RunFFmpeg(
		"-y",
		"-i", baseOutput,
		"-vf", filter,
		"-c:v", "libx264",
		"-preset", x264Preset,
		"-pix_fmt", "yuv420p",
		"-c:a", "copy",
		input.OutputPath,
	); err != nil {
		return err
	}
	_ = os.Remove(baseOutput)
	if err := prependPodcastIntroIfNeeded(input, x264Preset); err != nil {
		return err
	}
	return nil
}

func podcastFontsDir() string {
	candidates := []string{
		filepath.Join("assets", "fonts"),
		filepath.Join("worker", "assets", "fonts"),
		"/Users/luca/go/github.com/luca/video-saas/worker/assets/fonts",
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
	}
	return filepath.Join("worker", "assets", "fonts")
}

func escapeFFmpegPath(path string) string {
	path = strings.ReplaceAll(path, `\`, `\\`)
	path = strings.ReplaceAll(path, ":", `\:`)
	return path
}

func podcastX264Preset() string {
	return strings.TrimSpace(conf.Get[string]("worker.podcast_x264_preset", "veryfast"))
}

func prependPodcastIntroIfNeeded(input ComposeInput, x264Preset string) error {
	if input.Script == nil || strings.TrimSpace(input.Script.Language) != "zh" {
		return nil
	}

	introPath := podcastIntroAnimationPath("zh")
	if strings.TrimSpace(introPath) == "" {
		return nil
	}
	if _, err := os.Stat(introPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	introDurationSec, err := common.AudioDurationSec(introPath)
	if err != nil {
		return err
	}
	if introDurationSec <= 0 {
		return nil
	}

	tempOutput := strings.TrimSuffix(input.OutputPath, filepath.Ext(input.OutputPath)) + "_with_intro.mp4"
	scale := common.ResolutionToScale(input.Resolution)
	filter := fmt.Sprintf("[0:v]scale=%s,setsar=1[v0];[1:v]setsar=1[v1];[v0][2:a][v1][1:a]concat=n=2:v=1:a=1[v][a]", scale)
	if err := common.RunFFmpeg(
		"-y",
		"-i", introPath,
		"-i", input.OutputPath,
		"-f", "lavfi",
		"-t", fmt.Sprintf("%.3f", introDurationSec),
		"-i", "anullsrc=r=48000:cl=stereo",
		"-filter_complex", filter,
		"-map", "[v]",
		"-map", "[a]",
		"-c:v", "libx264",
		"-preset", x264Preset,
		"-pix_fmt", "yuv420p",
		"-c:a", "aac",
		tempOutput,
	); err != nil {
		return err
	}

	if err := os.Remove(input.OutputPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return os.Rename(tempOutput, input.OutputPath)
}

func podcastIntroAnimationPath(language string) string {
	language = strings.TrimSpace(strings.ToLower(language))
	if language == "" {
		return ""
	}
	candidates := []string{
		filepath.Join(conf.Get[string]("worker.ffmpeg_work_dir"), "podcast", "animation", language+"_open.mp4"),
		filepath.Join("artifacts", "podcast", "animation", language+"_open.mp4"),
		filepath.Join("/Users/luca/go/github.com/luca/video-saas/artifacts/podcast/animation", language+"_open.mp4"),
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return ""
}
