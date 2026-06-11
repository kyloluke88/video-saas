package pipeline

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	dto "worker/services/practical/model"
)

func DecodePayload(raw map[string]interface{}) (dto.PracticalAudioGeneratePayload, error) {
	data, err := json.Marshal(raw)
	if err != nil {
		return dto.PracticalAudioGeneratePayload{}, err
	}
	var payload dto.PracticalAudioGeneratePayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return dto.PracticalAudioGeneratePayload{}, err
	}
	return payload, nil
}

func PersistPayload(payload dto.PracticalAudioGeneratePayload) error {
	projectDir := practicalProjectDir(payload.ProjectID)
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(projectDir, "request_payload.json"), data, 0o644)
}

func LoadPayload(projectID string) (dto.PracticalAudioGeneratePayload, error) {
	if strings.TrimSpace(projectID) == "" {
		return dto.PracticalAudioGeneratePayload{}, fmt.Errorf("project_id is required")
	}
	rawPayload, err := os.ReadFile(filepath.Join(practicalProjectDir(projectID), "request_payload.json"))
	if err != nil {
		return dto.PracticalAudioGeneratePayload{}, err
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(rawPayload, &raw); err != nil {
		return dto.PracticalAudioGeneratePayload{}, fmt.Errorf("project request payload invalid for %s: %w", projectID, err)
	}
	payload, err := DecodePayload(raw)
	if err != nil {
		return dto.PracticalAudioGeneratePayload{}, err
	}
	if strings.TrimSpace(payload.ProjectID) == "" {
		payload.ProjectID = strings.TrimSpace(projectID)
	}
	return payload, nil
}

func ResolvePayload(current dto.PracticalAudioGeneratePayload) (dto.PracticalAudioGeneratePayload, error) {
	if current.RunMode == 0 {
		return normalizePayload(current)
	}

	saved, err := LoadPayload(current.ProjectID)
	if err != nil {
		return dto.PracticalAudioGeneratePayload{}, err
	}
	merged, err := MergePayload(saved, current)
	if err != nil {
		return dto.PracticalAudioGeneratePayload{}, err
	}
	return normalizePayload(merged)
}

func MergePayload(saved, current dto.PracticalAudioGeneratePayload) (dto.PracticalAudioGeneratePayload, error) {
	targetProjectID := strings.TrimSpace(current.ProjectID)
	if targetProjectID == "" {
		return dto.PracticalAudioGeneratePayload{}, fmt.Errorf("project_id is required")
	}

	lang := strings.ToLower(strings.TrimSpace(saved.Lang))
	currentLang := strings.ToLower(strings.TrimSpace(current.Lang))
	if currentLang != "" && lang != "" && currentLang != lang {
		return dto.PracticalAudioGeneratePayload{}, fmt.Errorf("lang mismatch for rerun requested=%s project=%s", currentLang, lang)
	}
	if lang == "" {
		lang = currentLang
	}
	if !validPracticalLanguage(lang) {
		return dto.PracticalAudioGeneratePayload{}, fmt.Errorf("lang must be zh or ja")
	}

	return dto.PracticalAudioGeneratePayload{
		ProjectID:      targetProjectID,
		Lang:           lang,
		TTSType:        normalizeTTSType(firstPositive(current.TTSType, saved.TTSType, 1)),
		RunMode:        firstPositive(current.RunMode, saved.RunMode),
		StartFrom:      strings.TrimSpace(current.StartFrom),
		StopAt:         strings.TrimSpace(current.StopAt),
		BlockNums:      compactPositiveInts(current.BlockNums),
		ChapterNums:    compactPositiveInts(current.ChapterNums),
		ScriptFilename: firstNonEmpty(current.ScriptFilename, saved.ScriptFilename),
		Resolution:     firstNonEmpty(current.Resolution, saved.Resolution),
		AspectRatio:    firstNonEmpty(current.AspectRatio, saved.AspectRatio),
		DesignType:     normalizeDesignType(firstPositive(current.DesignType, saved.DesignType, 1)),
	}, nil
}

func normalizePayload(payload dto.PracticalAudioGeneratePayload) (dto.PracticalAudioGeneratePayload, error) {
	payload.ProjectID = strings.TrimSpace(payload.ProjectID)
	payload.Lang = strings.ToLower(strings.TrimSpace(payload.Lang))
	payload.TTSType = normalizeTTSType(payload.TTSType)
	payload.RunMode = normalizeRunMode(payload.RunMode)
	payload.StartFrom = strings.TrimSpace(payload.StartFrom)
	payload.StopAt = strings.TrimSpace(payload.StopAt)
	payload.BlockNums = compactPositiveInts(payload.BlockNums)
	payload.ChapterNums = compactPositiveInts(payload.ChapterNums)
	payload.ScriptFilename = strings.TrimSpace(payload.ScriptFilename)
	payload.Resolution = strings.TrimSpace(payload.Resolution)
	payload.AspectRatio = strings.TrimSpace(payload.AspectRatio)
	payload.DesignType = normalizeDesignType(payload.DesignType)
	_, _, err := ValidateRange(payload.TTSType, payload.RunMode, payload.StartFrom, payload.StopAt)
	if err != nil {
		return dto.PracticalAudioGeneratePayload{}, err
	}
	return payload, nil
}

func normalizeRunMode(value int) int {
	if value == 1 {
		return 1
	}
	return 0
}

func normalizeTTSType(value int) int {
	return NormalizeTTSType(value)
}

func normalizeDesignType(value int) int {
	if value == 2 {
		return 2
	}
	return 1
}

func validPracticalLanguage(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "zh", "ja":
		return true
	default:
		return false
	}
}

func firstPositive(values ...int) int {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func compactPositiveInts(values []int) []int {
	seen := make(map[int]struct{}, len(values))
	out := make([]int, 0, len(values))
	for _, value := range values {
		if value <= 0 {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func practicalProjectDir(projectID string) string {
	return filepath.Join(practicalOutputsRoot(), "projects", strings.TrimSpace(projectID))
}

func practicalOutputsRoot() string {
	candidates := []string{
		"outputs",
		filepath.Join("worker", "outputs"),
		"/app/outputs",
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return practicalAbsolutePath(candidate)
		}
	}
	return practicalAbsolutePath(candidates[0])
}

func practicalAbsolutePath(path string) string {
	if filepath.IsAbs(strings.TrimSpace(path)) {
		return strings.TrimSpace(path)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	return abs
}
