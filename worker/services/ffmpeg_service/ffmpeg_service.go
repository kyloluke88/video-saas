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
	"worker/pkg/helpers"
	"worker/pkg/tts"
	services "worker/services"
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

func BurnSubtitles(input string, plan services.ProjectPlanResult, output string) error {
	srt := buildSRT(plan.Scenes)
	if strings.TrimSpace(srt) == "" {
		return RunFFmpeg("-y", "-i", input, "-c", "copy", output)
	}
	projectDir := filepath.Dir(input)
	srtPath := filepath.Join(projectDir, "subtitles.srt")
	if err := os.WriteFile(srtPath, []byte(srt), 0o644); err != nil {
		return err
	}

	style := subtitleStyleForPlan(plan)
	filter := fmt.Sprintf("subtitles=%s:force_style='%s'", escapeFFmpegPath(srtPath), style)
	return RunFFmpeg("-y", "-i", input, "-vf", filter, "-c:a", "copy", output)
}

func buildSRT(scenes []services.ScenePlan) string {
	var b strings.Builder
	cursor := 0
	index := 1
	for _, scene := range scenes {
		text := strings.TrimSpace(scene.Narration)
		start := formatSRTTime(cursor)
		cursor += helpers.Max(1, scene.DurationSec)
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

func SynthesizeNarrationAudio(plan services.ProjectPlanResult, projectDir string) (string, error) {
	if plan.NarrationFull == "" {
		return "", nil
	}
	if !conf.Get[bool]("worker.idiom_tts_enabled") {
		return "", nil
	}

	provider, err := tts.NewProvider(providerConfigForLanguage(plan.NarrationLanguage))
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

func providerConfigForLanguage(language string) tts.Config {
	switch strings.ToLower(strings.TrimSpace(language)) {
	case "ja", "ja-jp":
		return tts.Config{
			Provider:               "elevenlabs",
			ElevenLabsBaseURL:      conf.Get[string]("worker.elevenlabs_tts_base_url"),
			ElevenLabsAPIKey:       conf.Get[string]("worker.elevenlabs_tts_api_key"),
			ElevenLabsVoiceID:      conf.Get[string]("worker.elevenlabs_tts_voice_id"),
			ElevenLabsModelID:      conf.Get[string]("worker.elevenlabs_tts_model_id"),
			ElevenLabsOutputFormat: conf.Get[string]("worker.elevenlabs_tts_output_format"),
			ElevenLabsEnableLog:    conf.Get[bool]("worker.elevenlabs_tts_enable_logging"),
		}
	default:
		return tts.Config{
			Provider:               "tencent",
			TencentRegion:          conf.Get[string]("worker.tencent_tts_region"),
			TencentSecretID:        conf.Get[string]("worker.tencent_tts_secret_id"),
			TencentSecretKey:       conf.Get[string]("worker.tencent_tts_secret_key"),
			TencentVoiceType:       conf.Get[int64]("worker.tencent_tts_voice_type"),
			TencentPrimaryLanguage: conf.Get[int64]("worker.tencent_tts_primary_language"),
			TencentModelType:       conf.Get[int64]("worker.tencent_tts_model_type"),
			TencentCodec:           conf.Get[string]("worker.tencent_tts_codec"),
		}
	}
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

func ComposeFinalVideo(plan services.ProjectPlanResult, videoInput, narrationAudio, bgmPath, finalOutput string) error {
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
		return RunFFmpeg(args...)
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
		return RunFFmpeg(args...)
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
		return RunFFmpeg(args...)
	}

	args := []string{"-y", "-i", videoInput}
	args = append(args, encodeArgs...)
	args = append(args, finalOutput)
	return RunFFmpeg(args...)
}

func NormalizeSceneVideo(inputPath, outputPath string) error {
	return RunFFmpeg(
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
	if targetSec <= 0 {
		return NormalizeSceneVideo(inputPath, outputPath)
	}
	return RunFFmpeg(
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
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(conf.Get[int]("worker.ffmpeg_timeout_sec"))*time.Second)
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

func subtitleStyleForPlan(plan services.ProjectPlanResult) string {
	return profileForPlan(plan).SubtitleStyle
}

func profileForPlan(plan services.ProjectPlanResult) ffmpegPlatformProfile {
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
