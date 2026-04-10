package ffmpeg_service

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	conf "worker/pkg/config"
	idiommodel "worker/services/idiom/model"
)

type ffmpegPlatformProfile struct {
	SubtitleStyle string
	BGMVolume     float64
	VideoCodec    string
	VideoPreset   string
	PixFmt        string
	Movflags      string
	AudioCodec    string
	CRF           string
}

// platformProfiles is the single source of truth for platform-specific post settings.
// Tune values here to adjust final output behavior without touching business flow.
var platformProfiles = map[string]ffmpegPlatformProfile{
	"tiktok": {
		SubtitleStyle: "FontName=Noto Sans CJK SC,FontSize=16,Outline=2,Shadow=0,MarginV=40",
		BGMVolume:     0.16,
		VideoCodec:    "libx264",
		VideoPreset:   "faster",
		PixFmt:        "yuv420p",
		Movflags:      "+faststart",
		AudioCodec:    "aac",
		CRF:           "23",
	},
	"youtube": {
		SubtitleStyle: "FontName=Noto Sans CJK SC,FontSize=16,Outline=2,Shadow=0,MarginV=36",
		BGMVolume:     0.14,
		VideoCodec:    "libx264",
		VideoPreset:   "medium",
		PixFmt:        "yuv420p",
		Movflags:      "+faststart",
		AudioCodec:    "aac",
		CRF:           "22",
	},
	"both": {
		SubtitleStyle: "FontName=Noto Sans CJK SC,FontSize=16,Outline=2,Shadow=0,MarginV=36",
		BGMVolume:     0.15,
		VideoCodec:    "libx264",
		VideoPreset:   "medium",
		PixFmt:        "yuv420p",
		Movflags:      "+faststart",
		AudioCodec:    "aac",
		CRF:           "23",
	},
}

func BurnSubtitles(input string, plan idiommodel.RenderPlan, output string) error {
	return BurnSubtitlesContext(context.Background(), input, plan, output)
}

func BurnSubtitlesContext(ctx context.Context, input string, plan idiommodel.RenderPlan, output string) error {
	srt := buildSRT(plan.Scenes)
	if strings.TrimSpace(srt) == "" {
		return RunFFmpegContext(ctx, "-y", "-i", input, "-c", "copy", output)
	}
	projectDir := filepath.Dir(input)
	srtPath := filepath.Join(projectDir, "subtitles.srt")
	if err := os.WriteFile(srtPath, []byte(srt), 0o644); err != nil {
		return err
	}

	style := subtitleStyleForPlan(plan)
	filter := fmt.Sprintf("subtitles=%s:force_style='%s'", escapeFFmpegPath(srtPath), style)
	return RunFFmpegContext(ctx, "-y", "-i", input, "-vf", filter, "-c:a", "copy", output)
}

func buildSRT(scenes []idiommodel.RenderScene) string {
	var b strings.Builder
	cursor := 0
	index := 1
	for _, scene := range scenes {
		text := strings.TrimSpace(scene.Narration)
		start := formatSRTTime(cursor)
		cursor += maxInt(1, scene.DurationSec)
		end := formatSRTTime(cursor)
		if text == "" {
			continue
		}
		b.WriteString(fmt.Sprintf("%d\n%s --> %s\n%s\n\n", index, start, end, text))
		index++
	}
	return b.String()
}

func formatSRTTime(sec int) string {
	h := sec / 3600
	m := (sec % 3600) / 60
	s := sec % 60
	return fmt.Sprintf("%02d:%02d:%02d,000", h, m, s)
}

func SelectRandomBGM() (string, error) {
	bgmDir := filepath.Join(conf.Get[string]("worker.ffmpeg_work_dir"), "bgm")
	entries, err := os.ReadDir(bgmDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", err
	}

	candidates := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		switch ext {
		case ".mp3", ".wav", ".m4a", ".aac", ".flac", ".ogg":
			candidates = append(candidates, filepath.Join(bgmDir, entry.Name()))
		}
	}
	if len(candidates) == 0 {
		return "", nil
	}

	max := big.NewInt(int64(len(candidates)))
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		return "", err
	}
	return candidates[n.Int64()], nil
}

func ComposeFinalVideo(plan idiommodel.RenderPlan, videoInput, narrationAudio, bgmPath, finalOutput string) error {
	return ComposeFinalVideoContext(context.Background(), plan, videoInput, narrationAudio, bgmPath, finalOutput)
}

func ComposeFinalVideoContext(ctx context.Context, plan idiommodel.RenderPlan, videoInput, narrationAudio, bgmPath, finalOutput string) error {
	profile := profileForPlan(plan)
	bgmVolume := profile.BGMVolume
	encodeArgs := finalEncodeArgs(profile)
	if bgmPath != "" && narrationAudio != "" {
		args := []string{
			"-y",
			"-i", videoInput,
			"-stream_loop", "-1",
			"-i", bgmPath,
			"-i", narrationAudio,
			"-filter_complex", fmt.Sprintf("[1:a]volume=%.2f[bgm];[2:a]volume=1.0[narr];[bgm][narr]amix=inputs=2:duration=longest:dropout_transition=2[aout]", bgmVolume),
			"-map", "0:v:0",
			"-map", "[aout]",
			"-shortest",
		}
		args = append(args, encodeArgs...)
		args = append(args, finalOutput)
		return RunFFmpegContext(ctx, args...)
	}

	if bgmPath != "" {
		args := []string{
			"-y",
			"-i", videoInput,
			"-stream_loop", "-1",
			"-i", bgmPath,
			"-map", "0:v:0",
			"-map", "1:a:0",
			"-shortest",
		}
		args = append(args, encodeArgs...)
		args = append(args, finalOutput)
		return RunFFmpegContext(ctx, args...)
	}

	if narrationAudio != "" {
		args := []string{
			"-y",
			"-i", videoInput,
			"-i", narrationAudio,
			"-shortest",
		}
		args = append(args, encodeArgs...)
		args = append(args, finalOutput)
		return RunFFmpegContext(ctx, args...)
	}

	args := []string{"-y", "-i", videoInput}
	args = append(args, encodeArgs...)
	args = append(args, finalOutput)
	return RunFFmpegContext(ctx, args...)
}

func NormalizeSceneVideo(inputPath, outputPath string) error {
	return NormalizeSceneVideoContext(context.Background(), inputPath, outputPath)
}

func NormalizeSceneVideoContext(ctx context.Context, inputPath, outputPath string) error {
	return RunFFmpegContext(
		ctx,
		"-y",
		"-i", inputPath,
		"-c:v", "libx264",
		"-preset", "medium",
		"-movflags", "+faststart",
		"-pix_fmt", "yuv420p",
		"-c:a", "aac",
		outputPath,
	)
}

func TrimVideoDuration(inputPath, outputPath string, targetSec int) error {
	return TrimVideoDurationContext(context.Background(), inputPath, outputPath, targetSec)
}

func TrimVideoDurationContext(ctx context.Context, inputPath, outputPath string, targetSec int) error {
	if targetSec <= 0 {
		return NormalizeSceneVideoContext(ctx, inputPath, outputPath)
	}
	return RunFFmpegContext(
		ctx,
		"-y",
		"-i", inputPath,
		"-t", fmt.Sprintf("%d", targetSec),
		"-c:v", "libx264",
		"-preset", "medium",
		"-movflags", "+faststart",
		"-pix_fmt", "yuv420p",
		"-c:a", "aac",
		outputPath,
	)
}

func RunFFmpeg(args ...string) error {
	return RunFFmpegContext(context.Background(), args...)
}

func RunFFmpegContext(ctx context.Context, args ...string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(conf.Get[int]("worker.ffmpeg_timeout_sec"))*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg error: %w output=%s", err, string(out))
	}
	return nil
}

func escapeFFmpegPath(path string) string {
	path = strings.ReplaceAll(path, `\`, `\\`)
	path = strings.ReplaceAll(path, ":", `\:`)
	return path
}

func subtitleStyleForPlan(plan idiommodel.RenderPlan) string {
	return profileForPlan(plan).SubtitleStyle
}

func profileForPlan(plan idiommodel.RenderPlan) ffmpegPlatformProfile {
	platform := strings.ToLower(strings.TrimSpace(plan.Platform))
	if platform == "" {
		platform = "both"
	}
	if p, ok := platformProfiles[platform]; ok {
		return p
	}
	return platformProfiles["both"]
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func finalEncodeArgs(p ffmpegPlatformProfile) []string {
	args := []string{
		"-c:v", p.VideoCodec,
		"-preset", p.VideoPreset,
		"-pix_fmt", p.PixFmt,
		"-movflags", p.Movflags,
		"-c:a", p.AudioCodec,
	}
	if strings.TrimSpace(p.CRF) != "" {
		args = append(args, "-crf", p.CRF)
	}
	return args
}
