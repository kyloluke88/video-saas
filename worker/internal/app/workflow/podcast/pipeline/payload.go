package pipeline

import (
	"encoding/json"
	"fmt"
	"strings"

	"worker/internal/workspace"
	dto "worker/services/podcast/model"
)

func DecodeGeneratePayload(raw map[string]interface{}) (dto.PodcastAudioGeneratePayload, error) {
	data, err := json.Marshal(raw)
	if err != nil {
		return dto.PodcastAudioGeneratePayload{}, err
	}
	var payload dto.PodcastAudioGeneratePayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return dto.PodcastAudioGeneratePayload{}, err
	}
	return payload, nil
}

func DecodeComposePayload(raw map[string]interface{}) (dto.PodcastComposePayload, error) {
	data, err := json.Marshal(raw)
	if err != nil {
		return dto.PodcastComposePayload{}, err
	}
	var payload dto.PodcastComposePayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return dto.PodcastComposePayload{}, err
	}
	return payload, nil
}

func PersistGeneratePayload(payload dto.PodcastAudioGeneratePayload) error {
	return workspace.WriteRequestPayload(payload.ProjectID, payload)
}

func LoadGeneratePayload(projectID string) (dto.PodcastAudioGeneratePayload, error) {
	if strings.TrimSpace(projectID) == "" {
		return dto.PodcastAudioGeneratePayload{}, fmt.Errorf("project_id is required")
	}
	rawPayload, err := workspace.LoadRequestPayloadMap(projectID)
	if err != nil {
		return dto.PodcastAudioGeneratePayload{}, err
	}
	payload, err := DecodeGeneratePayload(rawPayload)
	if err != nil {
		return dto.PodcastAudioGeneratePayload{}, err
	}
	if strings.TrimSpace(payload.ProjectID) == "" {
		payload.ProjectID = strings.TrimSpace(projectID)
	}
	return payload, nil
}

func ResolveGeneratePayload(current dto.PodcastAudioGeneratePayload) (dto.PodcastAudioGeneratePayload, error) {
	if current.RunMode == 0 {
		return normalizeGeneratePayload(current)
	}

	saved, err := LoadGeneratePayload(current.ProjectID)
	if err != nil {
		return dto.PodcastAudioGeneratePayload{}, err
	}
	merged, err := MergeGeneratePayload(saved, current)
	if err != nil {
		return dto.PodcastAudioGeneratePayload{}, err
	}
	return normalizeGeneratePayload(merged)
}

func ResolveComposePayload(current dto.PodcastComposePayload) (dto.PodcastComposePayload, error) {
	if current.RunMode == 0 {
		return normalizeComposePayload(current)
	}

	saved, err := LoadGeneratePayload(current.ProjectID)
	if err != nil {
		return dto.PodcastComposePayload{}, err
	}
	base, err := BuildComposePayloadFromGenerate(saved)
	if err != nil {
		return dto.PodcastComposePayload{}, err
	}
	merged := base
	if current.ProjectID != "" {
		merged.ProjectID = strings.TrimSpace(current.ProjectID)
	}
	if current.Lang != "" {
		merged.Lang = strings.TrimSpace(current.Lang)
	}
	if current.TTSType > 0 {
		merged.TTSType = current.TTSType
	}
	if current.RunMode > 0 {
		merged.RunMode = current.RunMode
	}
	if current.StartFrom != "" {
		merged.StartFrom = strings.TrimSpace(current.StartFrom)
	}
	if current.StopAt != "" {
		merged.StopAt = strings.TrimSpace(current.StopAt)
	}
	if len(current.BgImgFilenames) > 0 {
		merged.BgImgFilenames = compactNonEmptyStrings(current.BgImgFilenames)
	}
	if current.TargetPlatform != "" {
		merged.TargetPlatform = strings.TrimSpace(current.TargetPlatform)
	}
	if current.AspectRatio != "" {
		merged.AspectRatio = strings.TrimSpace(current.AspectRatio)
	}
	if current.Resolution != "" {
		merged.Resolution = strings.TrimSpace(current.Resolution)
	}
	if current.DesignStyle > 0 {
		merged.DesignStyle = current.DesignStyle
	}
	if current.VideoURL != "" {
		merged.VideoURL = strings.TrimSpace(current.VideoURL)
	}
	if current.YouTubeVideoID != "" {
		merged.YouTubeVideoID = strings.TrimSpace(current.YouTubeVideoID)
	}
	if current.YouTubeVideoURL != "" {
		merged.YouTubeVideoURL = strings.TrimSpace(current.YouTubeVideoURL)
	}
	return normalizeComposePayload(merged)
}

func MergeGeneratePayload(saved, current dto.PodcastAudioGeneratePayload) (dto.PodcastAudioGeneratePayload, error) {
	targetProjectID := strings.TrimSpace(current.ProjectID)
	if targetProjectID == "" {
		return dto.PodcastAudioGeneratePayload{}, fmt.Errorf("project_id is required")
	}

	lang := strings.ToLower(strings.TrimSpace(saved.Lang))
	currentLang := strings.ToLower(strings.TrimSpace(current.Lang))
	if currentLang != "" && lang != "" && currentLang != lang {
		return dto.PodcastAudioGeneratePayload{}, fmt.Errorf("lang mismatch for rerun requested=%s project=%s", currentLang, lang)
	}
	if lang == "" {
		lang = currentLang
	}
	if !validPodcastLanguage(lang) {
		return dto.PodcastAudioGeneratePayload{}, fmt.Errorf("lang must be zh or ja")
	}

	return dto.PodcastAudioGeneratePayload{
		ProjectID:       targetProjectID,
		Lang:            lang,
		TTSType:         firstPositive(current.TTSType, saved.TTSType, 1),
		IsMultiple:      firstOptionalInt(current.IsMultiple, saved.IsMultiple, intPtr(1)),
		Seed:            firstPositive(current.Seed, saved.Seed),
		RunMode:         firstPositive(current.RunMode, saved.RunMode),
		StartFrom:       strings.TrimSpace(current.StartFrom),
		StopAt:          strings.TrimSpace(current.StopAt),
		BlockNums:       compactPositiveInts(current.BlockNums),
		ScriptFilename:  firstNonEmpty(current.ScriptFilename, saved.ScriptFilename),
		BgImgFilenames:  firstNonEmptyStrings(current.BgImgFilenames, saved.BgImgFilenames),
		TargetPlatform:  firstNonEmpty(current.TargetPlatform, saved.TargetPlatform),
		AspectRatio:     firstNonEmpty(current.AspectRatio, saved.AspectRatio),
		Resolution:      firstNonEmpty(current.Resolution, saved.Resolution),
		DesignStyle:     normalizePodcastDesignStyle(firstPositive(current.DesignStyle, saved.DesignStyle, 1)),
		VideoURL:        firstNonEmpty(current.VideoURL, saved.VideoURL),
		YouTubeVideoID:  firstNonEmpty(current.YouTubeVideoID, saved.YouTubeVideoID),
		YouTubeVideoURL: firstNonEmpty(current.YouTubeVideoURL, saved.YouTubeVideoURL),
	}, nil
}

func BuildComposePayloadFromGenerate(generate dto.PodcastAudioGeneratePayload) (dto.PodcastComposePayload, error) {
	projectID := strings.TrimSpace(generate.ProjectID)
	if projectID == "" {
		return dto.PodcastComposePayload{}, fmt.Errorf("project_id is required")
	}
	lang := strings.ToLower(strings.TrimSpace(generate.Lang))
	if !validPodcastLanguage(lang) {
		return dto.PodcastComposePayload{}, fmt.Errorf("lang must be zh or ja")
	}

	backgrounds := firstNonEmptyStrings(generate.BgImgFilenames)
	if len(backgrounds) == 0 {
		return dto.PodcastComposePayload{}, fmt.Errorf("bg_img_filenames is required")
	}

	return dto.PodcastComposePayload{
		ProjectID:       projectID,
		Lang:            lang,
		TTSType:         firstPositive(generate.TTSType, 1),
		RunMode:         generate.RunMode,
		StartFrom:       strings.TrimSpace(generate.StartFrom),
		StopAt:          strings.TrimSpace(generate.StopAt),
		BgImgFilenames:  backgrounds,
		TargetPlatform:  strings.TrimSpace(generate.TargetPlatform),
		AspectRatio:     strings.TrimSpace(generate.AspectRatio),
		Resolution:      strings.TrimSpace(generate.Resolution),
		DesignStyle:     normalizePodcastDesignStyle(generate.DesignStyle),
		VideoURL:        strings.TrimSpace(generate.VideoURL),
		YouTubeVideoID:  strings.TrimSpace(generate.YouTubeVideoID),
		YouTubeVideoURL: strings.TrimSpace(generate.YouTubeVideoURL),
	}, nil
}

func normalizeGeneratePayload(payload dto.PodcastAudioGeneratePayload) (dto.PodcastAudioGeneratePayload, error) {
	payload.ProjectID = strings.TrimSpace(payload.ProjectID)
	payload.Lang = strings.ToLower(strings.TrimSpace(payload.Lang))
	payload.TTSType = NormalizeTTSType(payload.TTSType)
	payload.RunMode = normalizeRunMode(payload.RunMode)
	payload.StartFrom = strings.TrimSpace(payload.StartFrom)
	payload.StopAt = strings.TrimSpace(payload.StopAt)
	payload.BlockNums = compactPositiveInts(payload.BlockNums)
	payload.ScriptFilename = strings.TrimSpace(payload.ScriptFilename)
	payload.BgImgFilenames = compactNonEmptyStrings(payload.BgImgFilenames)
	payload.TargetPlatform = strings.TrimSpace(payload.TargetPlatform)
	payload.AspectRatio = strings.TrimSpace(payload.AspectRatio)
	payload.Resolution = strings.TrimSpace(payload.Resolution)
	payload.DesignStyle = normalizePodcastDesignStyle(payload.DesignStyle)
	payload.VideoURL = strings.TrimSpace(payload.VideoURL)
	payload.YouTubeVideoID = strings.TrimSpace(payload.YouTubeVideoID)
	payload.YouTubeVideoURL = strings.TrimSpace(payload.YouTubeVideoURL)
	_, _, err := ValidateRange(payload.TTSType, payload.RunMode, payload.StartFrom, payload.StopAt)
	if err != nil {
		return dto.PodcastAudioGeneratePayload{}, err
	}
	return payload, nil
}

func normalizeComposePayload(payload dto.PodcastComposePayload) (dto.PodcastComposePayload, error) {
	payload.ProjectID = strings.TrimSpace(payload.ProjectID)
	payload.Lang = strings.ToLower(strings.TrimSpace(payload.Lang))
	payload.TTSType = NormalizeTTSType(payload.TTSType)
	payload.RunMode = normalizeRunMode(payload.RunMode)
	payload.StartFrom = strings.TrimSpace(payload.StartFrom)
	payload.StopAt = strings.TrimSpace(payload.StopAt)
	payload.BgImgFilenames = compactNonEmptyStrings(payload.BgImgFilenames)
	payload.TargetPlatform = strings.TrimSpace(payload.TargetPlatform)
	payload.AspectRatio = strings.TrimSpace(payload.AspectRatio)
	payload.Resolution = strings.TrimSpace(payload.Resolution)
	payload.DesignStyle = normalizePodcastDesignStyle(payload.DesignStyle)
	payload.VideoURL = strings.TrimSpace(payload.VideoURL)
	payload.YouTubeVideoID = strings.TrimSpace(payload.YouTubeVideoID)
	payload.YouTubeVideoURL = strings.TrimSpace(payload.YouTubeVideoURL)
	_, _, err := ValidateRange(payload.TTSType, payload.RunMode, payload.StartFrom, payload.StopAt)
	if err != nil {
		return dto.PodcastComposePayload{}, err
	}
	return payload, nil
}

func normalizeRunMode(value int) int {
	if value == 1 {
		return 1
	}
	return 0
}

func validPodcastLanguage(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "zh", "ja":
		return true
	default:
		return false
	}
}

func normalizePodcastDesignStyle(value int) int {
	if value == 2 {
		return 2
	}
	return 1
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func firstPositive(values ...int) int {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func firstOptionalInt(values ...*int) *int {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func intPtr(value int) *int {
	v := value
	return &v
}

func firstNonEmptyStrings(values ...[]string) []string {
	for _, group := range values {
		cleaned := compactNonEmptyStrings(group)
		if len(cleaned) > 0 {
			return cleaned
		}
	}
	return nil
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

func compactNonEmptyStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}
