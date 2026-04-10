package common

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

func AudioDurationSec(audioPath string) (float64, error) {
	return AudioDurationSecContext(context.Background(), audioPath)
}

func AudioDurationSecContext(ctx context.Context, audioPath string) (float64, error) {
	out, err := RunFFprobeContext(
		ctx,
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		audioPath,
	)
	if err != nil {
		return 0, err
	}
	trimmed := strings.TrimSpace(out)
	if trimmed == "" {
		return 0, fmt.Errorf("empty ffprobe duration output")
	}
	sec, err := strconv.ParseFloat(trimmed, 64)
	if err != nil {
		return 0, fmt.Errorf("parse duration failed: %w output=%s", err, trimmed)
	}
	return sec, nil
}

func ResolutionToScale(resolution string) string {
	switch strings.TrimSpace(strings.ToLower(resolution)) {
	case "480p":
		return "854:480"
	case "720p":
		return "1280:720"
	case "1440p":
		return "2560:1440"
	case "2000p":
		return "3556:2000"
	default:
		return "1920:1080"
	}
}
