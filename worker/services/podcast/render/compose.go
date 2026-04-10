package podcast

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	conf "worker/pkg/config"
	"worker/services/media/ffmpeg/common"
)

func ComposeVideo(input ComposeInput) error {
	return ComposeVideoContext(context.Background(), input)
}

func ComposeVideoContext(ctx context.Context, input ComposeInput) error {
	if err := ComposeBaseVideoContext(ctx, input); err != nil {
		return err
	}
	return FinalizeComposedVideoContext(ctx, input)
}

func ComposeBaseVideoContext(ctx context.Context, input ComposeInput) error {
	primaryBackgroundPath := strings.TrimSpace(input.BackgroundImagePath)
	if primaryBackgroundPath == "" {
		return fmt.Errorf("background image is required")
	}
	if strings.TrimSpace(input.DialogueAudioPath) == "" {
		return fmt.Errorf("dialogue audio is required")
	}
	if strings.TrimSpace(input.OutputPath) == "" {
		return fmt.Errorf("output path is required")
	}

	projectDir := filepath.Dir(input.OutputPath)
	audioInputIndex := 1
	wave := waveformPresetFor(input.Resolution, input.DesignStyle, audioInputIndex)
	x264Preset := podcastX264Preset()
	ffmpegTimeout := podcastComposeFFmpegTimeout(input.DialogueAudioPath)

	baseOutput := filepath.Join(projectDir, "podcast_base.mp4")
	bgFilter := backgroundGraphFor(input.Resolution)
	if strings.TrimSpace(wave.BackgroundFilter) != "" {
		bgFilter += "," + wave.BackgroundFilter
	}
	complexFilter := fmt.Sprintf("%s;%s;[bg][sw]overlay=%s[v]", bgFilter, wave.AudioGraph, wave.Overlay)
	args := []string{"-y"}
	args = append(args, "-loop", "1", "-i", primaryBackgroundPath)
	args = append(args,
		"-i", input.DialogueAudioPath,
		"-filter_complex", complexFilter,
		"-map", "[v]",
		"-map", fmt.Sprintf("%d:a:0", audioInputIndex),
		"-c:v", "libx264",
		"-preset", x264Preset,
		"-pix_fmt", "yuv420p",
		"-c:a", "aac",
		"-shortest",
		baseOutput,
	)
	return common.RunFFmpegWithTimeoutContext(ctx, ffmpegTimeout, args...)
}

func FinalizeComposedVideoContext(ctx context.Context, input ComposeInput) error {
	if strings.TrimSpace(input.DialogueAudioPath) == "" {
		return fmt.Errorf("dialogue audio is required")
	}
	if strings.TrimSpace(input.OutputPath) == "" {
		return fmt.Errorf("output path is required")
	}

	projectDir := filepath.Dir(input.OutputPath)
	baseOutput := filepath.Join(projectDir, "podcast_base.mp4")
	if _, err := os.Stat(baseOutput); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("podcast base video missing: %s", baseOutput)
		}
		return err
	}

	ffmpegTimeout := podcastComposeFFmpegTimeout(input.DialogueAudioPath)
	x264Preset := podcastX264Preset()

	if input.Script == nil {
		return prependPodcastIntroIfNeeded(ctx, input, baseOutput, x264Preset, ffmpegTimeout)
	}

	assPath, err := WritePodcastASS(*input.Script, projectDir, input.Resolution, input.DesignStyle)
	if err != nil {
		return err
	}
	if strings.TrimSpace(assPath) == "" {
		return prependPodcastIntroIfNeeded(ctx, input, baseOutput, x264Preset, ffmpegTimeout)
	}
	return renderFinalPodcastOutput(ctx, input, assPath, x264Preset, ffmpegTimeout)
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

func prependPodcastIntroIfNeeded(ctx context.Context, input ComposeInput, contentOutput, x264Preset string, ffmpegTimeout time.Duration) error {
	if strings.TrimSpace(contentOutput) == "" {
		return nil
	}
	if input.Script == nil {
		if contentOutput == input.OutputPath {
			return nil
		}
		return common.RunFFmpegWithTimeoutContext(ctx, ffmpegTimeout,
			"-y",
			"-i", contentOutput,
			"-c", "copy",
			input.OutputPath,
		)
	}
	language := strings.TrimSpace(strings.ToLower(input.Script.Language))
	if language != "zh" && language != "ja" {
		if contentOutput == input.OutputPath {
			return nil
		}
		return common.RunFFmpegWithTimeoutContext(ctx, ffmpegTimeout,
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
		return common.RunFFmpegWithTimeoutContext(ctx, ffmpegTimeout,
			"-y",
			"-i", contentOutput,
			"-c", "copy",
			input.OutputPath,
		)
	}
	if _, err := os.Stat(introPath); err != nil {
		if os.IsNotExist(err) {
			return common.RunFFmpegWithTimeoutContext(ctx, ffmpegTimeout,
				"-y",
				"-i", contentOutput,
				"-c", "copy",
				input.OutputPath,
			)
		}
		return err
	}

	introDurationSec, err := common.AudioDurationSecContext(ctx, introPath)
	if err != nil {
		return err
	}
	if introDurationSec <= 0 {
		return common.RunFFmpegWithTimeoutContext(ctx, ffmpegTimeout,
			"-y",
			"-i", contentOutput,
			"-c", "copy",
			input.OutputPath,
		)
	}

	scale := common.ResolutionToScale(input.Resolution)
	filter := fmt.Sprintf("[0:v]scale=%s,setsar=1[v0];[0:a]aresample=48000,asetpts=N/SR/TB[a0];[1:v]setsar=1[v1];[1:a]aresample=48000,asetpts=N/SR/TB[a1];[v0][a0][v1][a1]concat=n=2:v=1:a=1[v][a]", scale)
	if err := common.RunFFmpegWithTimeoutContext(ctx, ffmpegTimeout,
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

func renderFinalPodcastOutput(ctx context.Context, input ComposeInput, assPath, x264Preset string, ffmpegTimeout time.Duration) error {
	if strings.TrimSpace(assPath) == "" {
		return fmt.Errorf("subtitle path is required")
	}

	projectDir := filepath.Dir(input.OutputPath)
	baseOutput := filepath.Join(projectDir, "podcast_base.mp4")
	introPath := ""
	if input.Script != nil {
		introPath = podcastIntroAnimationPath(input.Script.Language)
	}
	filter := fmt.Sprintf("subtitles=%s:fontsdir=%s", escapeFFmpegPath(assPath), escapeFFmpegPath(podcastFontsDir()))

	if strings.TrimSpace(introPath) == "" {
		return common.RunFFmpegWithTimeoutContext(ctx, ffmpegTimeout,
			"-y",
			"-i", baseOutput,
			"-vf", filter,
			"-c:v", "libx264",
			"-preset", x264Preset,
			"-pix_fmt", "yuv420p",
			"-c:a", "copy",
			input.OutputPath,
		)
	}
	if _, err := os.Stat(introPath); err != nil {
		if os.IsNotExist(err) {
			return common.RunFFmpegWithTimeoutContext(ctx, ffmpegTimeout,
				"-y",
				"-i", baseOutput,
				"-vf", filter,
				"-c:v", "libx264",
				"-preset", x264Preset,
				"-pix_fmt", "yuv420p",
				"-c:a", "copy",
				input.OutputPath,
			)
		}
		return err
	}

	scale := common.ResolutionToScale(input.Resolution)
	complexFilter := fmt.Sprintf(
		"[0:v]scale=%s,setsar=1[v0];[0:a]aresample=48000,asetpts=N/SR/TB[a0];[1:v]%s,setsar=1[v1];[1:a]aresample=48000,asetpts=N/SR/TB[a1];[v0][a0][v1][a1]concat=n=2:v=1:a=1[v][a]",
		scale,
		filter,
	)
	return common.RunFFmpegWithTimeoutContext(ctx, ffmpegTimeout,
		"-y",
		"-i", introPath,
		"-i", baseOutput,
		"-filter_complex", complexFilter,
		"-map", "[v]",
		"-map", "[a]",
		"-c:v", "libx264",
		"-preset", x264Preset,
		"-pix_fmt", "yuv420p",
		"-c:a", "aac",
		input.OutputPath,
	)
}

func podcastComposeFFmpegTimeout(dialogueAudioPath string) time.Duration {
	configured := time.Duration(conf.Get[int]("worker.podcast_ffmpeg_timeout_sec")) * time.Second
	fallback := time.Duration(conf.Get[int]("worker.ffmpeg_timeout_sec", 300)) * time.Second
	durationSec := 0.0
	if measured, err := common.AudioDurationSec(dialogueAudioPath); err == nil && measured > 0 {
		durationSec = measured
	}
	return computePodcastComposeTimeout(configured, fallback, durationSec)
}

func computePodcastComposeTimeout(configured, fallback time.Duration, audioDurationSec float64) time.Duration {
	if configured > 0 {
		return configured
	}

	timeout := fallback
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}

	if audioDurationSec > 0 {
		estimated := time.Duration(audioDurationSec*float64(time.Second))*2 + 10*time.Minute
		if estimated > timeout {
			timeout = estimated
		}
	}

	if timeout < 20*time.Minute {
		timeout = 20 * time.Minute
	}
	if timeout > 2*time.Hour {
		timeout = 2 * time.Hour
	}
	return timeout
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
