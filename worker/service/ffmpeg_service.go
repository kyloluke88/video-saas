package service

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
	"worker/pkg/helpers"
	"worker/pkg/tts"
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
		SubtitleStyle: "FontName=Noto Sans CJK SC,FontSize=22,Outline=2,Shadow=0,MarginV=40",
		BGMVolume:     0.16,
		VideoCodec:    "libx264",
		VideoPreset:   "faster",
		PixFmt:        "yuv420p",
		Movflags:      "+faststart",
		AudioCodec:    "aac",
		CRF:           "23",
	},
	"youtube": {
		SubtitleStyle: "FontName=Noto Sans CJK SC,FontSize=20,Outline=2,Shadow=0,MarginV=36",
		BGMVolume:     0.14,
		VideoCodec:    "libx264",
		VideoPreset:   "medium",
		PixFmt:        "yuv420p",
		Movflags:      "+faststart",
		AudioCodec:    "aac",
		CRF:           "22",
	},
	"both": {
		SubtitleStyle: "FontName=Noto Sans CJK SC,FontSize=20,Outline=2,Shadow=0,MarginV=36",
		BGMVolume:     0.15,
		VideoCodec:    "libx264",
		VideoPreset:   "medium",
		PixFmt:        "yuv420p",
		Movflags:      "+faststart",
		AudioCodec:    "aac",
		CRF:           "23",
	},
}

func BurnSubtitles(cfg Config, input string, plan ProjectPlanResult, output string) error {
	srt := buildSRT(plan.Scenes)
	projectDir := filepath.Dir(input)
	srtPath := filepath.Join(projectDir, "subtitles.srt")
	if err := os.WriteFile(srtPath, []byte(srt), 0o644); err != nil {
		return err
	}

	style := subtitleStyleForPlan(plan)
	filter := fmt.Sprintf("subtitles=%s:force_style='%s'", escapeFFmpegPath(srtPath), style)
	return RunFFmpeg(cfg, "-y", "-i", input, "-vf", filter, "-c:a", "copy", output)
}

func buildSRT(scenes []ScenePlan) string {
	var b strings.Builder
	cursor := 0
	for i, scene := range scenes {
		text := scene.Narration
		if text == "" {
			text = fmt.Sprintf("Scene %d", scene.Index)
		}
		start := formatSRTTime(cursor)
		cursor += helpers.Max(1, scene.DurationSec)
		end := formatSRTTime(cursor)
		b.WriteString(fmt.Sprintf("%d\n%s --> %s\n%s\n\n", i+1, start, end, text))
	}
	return b.String()
}

func formatSRTTime(sec int) string {
	h := sec / 3600
	m := (sec % 3600) / 60
	s := sec % 60
	return fmt.Sprintf("%02d:%02d:%02d,000", h, m, s)
}

func SynthesizeNarrationAudio(cfg Config, plan ProjectPlanResult, projectDir string) (string, error) {
	if plan.NarrationFull == "" {
		return "", nil
	}

	provider, err := tts.NewProvider(tts.Config{
		Provider: cfg.TTSProvider,
		APIURL:   cfg.TTSAPIURL,
		APIKey:   cfg.TTSAPIKey,

		TencentRegion:          cfg.TTSTencentRegion,
		TencentSecretID:        cfg.TTSTencentSecretID,
		TencentSecretKey:       cfg.TTSTencentSecretKey,
		TencentVoiceType:       cfg.TTSTencentVoiceType,
		TencentPrimaryLanguage: cfg.TTSTencentPrimaryLanguage,
		TencentModelType:       cfg.TTSTencentModelType,
		TencentCodec:           cfg.TTSTencentCodec,
	})
	if err != nil {
		return "", err
	}

	result, err := provider.Synthesize(context.Background(), tts.Request{
		Text:     plan.NarrationFull,
		Language: strings.TrimSpace(plan.NarrationLanguage),
	})
	if err != nil {
		return "", err
	}
	if len(result.Audio) == 0 {
		return "", errors.New("tts returned empty audio")
	}

	ext := strings.TrimSpace(strings.ToLower(result.Ext))
	if ext == "" {
		ext = "mp3"
	}
	out := filepath.Join(projectDir, "narration."+ext)
	if err := os.WriteFile(out, result.Audio, 0o644); err != nil {
		return "", err
	}
	return out, nil
}

func SelectRandomBGM(cfg Config) (string, error) {
	bgmDir := filepath.Join(cfg.FFmpegWorkDir, "bgm")
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

func ComposeFinalVideo(cfg Config, plan ProjectPlanResult, videoInput, narrationAudio, bgmPath, finalOutput string) error {
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
		return RunFFmpeg(cfg, args...)
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
		return RunFFmpeg(cfg, args...)
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
		return RunFFmpeg(cfg, args...)
	}

	args := []string{"-y", "-i", videoInput}
	args = append(args, encodeArgs...)
	args = append(args, finalOutput)
	return RunFFmpeg(cfg, args...)
}

func NormalizeSceneVideo(cfg Config, inputPath, outputPath string) error {
	return RunFFmpeg(cfg,
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

func TrimVideoDuration(cfg Config, inputPath, outputPath string, targetSec int) error {
	if targetSec <= 0 {
		return NormalizeSceneVideo(cfg, inputPath, outputPath)
	}
	return RunFFmpeg(cfg,
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

func RunFFmpeg(cfg Config, args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.FFmpegTimeoutSec)*time.Second)
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

func subtitleStyleForPlan(plan ProjectPlanResult) string {
	return profileForPlan(plan).SubtitleStyle
}

func profileForPlan(plan ProjectPlanResult) ffmpegPlatformProfile {
	platform := strings.ToLower(strings.TrimSpace(plan.Platform))
	if platform == "" {
		platform = "both"
	}
	if p, ok := platformProfiles[platform]; ok {
		return p
	}
	return platformProfiles["both"]
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
