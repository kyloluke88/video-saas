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
	contentOutput := input.OutputPath
	if shouldKeepPodcastContentVideo(input) {
		contentOutput = filepath.Join(projectDir, "podcast_content.mp4")
	}
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
		return common.RunFFmpeg(
			"-y",
			"-i", baseOutput,
			"-c", "copy",
			input.OutputPath,
		)
	}

	assPath, err := WritePodcastASS(*input.Script, projectDir, input.Resolution, input.DesignStyle)
	if err != nil {
		return err
	}
	if strings.TrimSpace(assPath) == "" {
		return common.RunFFmpeg(
			"-y",
			"-i", baseOutput,
			"-c", "copy",
			input.OutputPath,
		)
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
		contentOutput,
	); err != nil {
		return err
	}
	if err := prependPodcastIntroIfNeeded(input, contentOutput, x264Preset); err != nil {
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

func prependPodcastIntroIfNeeded(input ComposeInput, contentOutput, x264Preset string) error {
	if input.Script == nil || strings.TrimSpace(contentOutput) == "" {
		return nil
	}
	language := strings.TrimSpace(strings.ToLower(input.Script.Language))
	if language != "zh" && language != "ja" {
		if contentOutput == input.OutputPath {
			return nil
		}
		return common.RunFFmpeg(
			"-y",
			"-i", contentOutput,
			"-c", "copy",
			input.OutputPath,
		)
	}

	if contentOutput == input.OutputPath {
		return nil
	}

	introPath := podcastIntroAnimationPath(language)
	if strings.TrimSpace(introPath) == "" {
		return common.RunFFmpeg(
			"-y",
			"-i", contentOutput,
			"-c", "copy",
			input.OutputPath,
		)
	}
	if _, err := os.Stat(introPath); err != nil {
		if os.IsNotExist(err) {
			return common.RunFFmpeg(
				"-y",
				"-i", contentOutput,
				"-c", "copy",
				input.OutputPath,
			)
		}
		return err
	}

	introDurationSec, err := common.AudioDurationSec(introPath)
	if err != nil {
		return err
	}
	if introDurationSec <= 0 {
		return common.RunFFmpeg(
			"-y",
			"-i", contentOutput,
			"-c", "copy",
			input.OutputPath,
		)
	}

	scale := common.ResolutionToScale(input.Resolution)
	filter := fmt.Sprintf("[0:v]scale=%s,setsar=1[v0];[0:a]aresample=48000,asetpts=N/SR/TB[a0];[1:v]setsar=1[v1];[1:a]aresample=48000,asetpts=N/SR/TB[a1];[v0][a0][v1][a1]concat=n=2:v=1:a=1[v][a]", scale)
	if err := common.RunFFmpeg(
		"-y",
		"-i", introPath,
		"-i", contentOutput,
		"-filter_complex", filter,
		"-map", "[v]",
		"-map", "[a]",
		"-c:v", "libx264",
		"-preset", x264Preset,
		"-pix_fmt", "yuv420p",
		"-c:a", "aac",
		input.OutputPath,
	); err != nil {
		return err
	}
	return nil
}

func shouldKeepPodcastContentVideo(input ComposeInput) bool {
	if input.Script == nil {
		return false
	}
	language := strings.TrimSpace(strings.ToLower(input.Script.Language))
	return language == "zh" || language == "ja"
}

func podcastIntroAnimationPath(language string) string {
	language = strings.TrimSpace(strings.ToLower(language))
	if language == "" {
		return ""
	}
	candidates := []string{
		filepath.Join(conf.Get[string]("worker.worker_assets_dir"), "podcast", "animation", language+"_open.mp4"),
		filepath.Join("assets", "podcast", "animation", language+"_open.mp4"),
		filepath.Join("worker", "assets", "podcast", "animation", language+"_open.mp4"),
		filepath.Join("/Users/luca/go/github.com/luca/video-saas/worker/assets/podcast/animation", language+"_open.mp4"),
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return ""
}
