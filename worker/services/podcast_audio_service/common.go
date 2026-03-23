package podcast_audio_service

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"worker/internal/dto"
	conf "worker/pkg/config"
)

func scriptPathFor(filename string) string {
	return filepath.Join(conf.Get[string]("worker.worker_assets_dir"), "podcast", "scripts", filepath.Base(strings.TrimSpace(filename)))
}

func projectScriptInputPath(projectDir string) string {
	return filepath.Join(projectDir, "script_input.json")
}

func projectDirFor(projectID string) string {
	return filepath.Join(conf.Get[string]("worker.ffmpeg_work_dir"), "projects", projectID)
}

func sanitizeSegmentID(segmentID string) string {
	raw := strings.TrimSpace(segmentID)
	if raw == "" {
		return "segment"
	}
	raw = strings.ReplaceAll(raw, "/", "-")
	raw = strings.ReplaceAll(raw, "\\", "-")
	return raw
}

func defaultSpeaker(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return "male"
	}
	return value
}

func writeJSON(path string, data interface{}) error {
	raw, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o644)
}

func readJSON(path string, out interface{}) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, out)
}

func copyFile(src, dst string) error {
	raw, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, raw, 0o644)
}

func isSilentToken(charText string) bool {
	rs := []rune(strings.TrimSpace(charText))
	if len(rs) != 1 {
		return false
	}
	return isPunctuationRune(rs[0])
}

func isPunctuationRune(r rune) bool {
	return strings.ContainsRune("，。！？；：“”‘’（）《》、…,.!?;:()[]{}\"'", r)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(values ...int) int {
	if len(values) == 0 {
		return 0
	}
	current := values[0]
	for _, value := range values[1:] {
		if value < current {
			current = value
		}
	}
	return current
}

func firstPositive(values ...int) int {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func alignedStats(script dto.PodcastScript) (int, int, int, int) {
	timedSegments := 0
	totalSegments := len(script.Segments)
	timedTokens := 0
	totalTokens := 0
	for _, seg := range script.Segments {
		if seg.EndMS > seg.StartMS {
			timedSegments++
		}
		for _, token := range seg.Tokens {
			totalTokens++
			if token.EndMS > token.StartMS {
				timedTokens++
			}
		}
	}
	return timedSegments, totalSegments, timedTokens, totalTokens
}
