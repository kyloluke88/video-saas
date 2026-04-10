package replay

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

func ResolveSourceProjectID(current dto.PodcastAudioGeneratePayload) (string, error) {
	if sourceProjectID := strings.TrimSpace(current.SourceProjectID); sourceProjectID != "" {
		return sourceProjectID, nil
	}
	return workspace.ReplaySourceProjectID(current.ProjectID)
}

func PrepareGeneratePayload(current dto.PodcastAudioGeneratePayload) (dto.PodcastAudioGeneratePayload, error) {
	sourceProjectID, err := ResolveSourceProjectID(current)
	if err != nil {
		return dto.PodcastAudioGeneratePayload{}, err
	}
	if err := workspace.EnsureReplayProjectDir(sourceProjectID, current.ProjectID); err != nil {
		return dto.PodcastAudioGeneratePayload{}, err
	}

	savedPayload, err := LoadGeneratePayload(sourceProjectID)
	if err != nil {
		return dto.PodcastAudioGeneratePayload{}, err
	}
	replayPayload, err := BuildGeneratePayloadFromSavedAndCurrent(savedPayload, current)
	if err != nil {
		return dto.PodcastAudioGeneratePayload{}, err
	}
	replayPayload.SourceProjectID = sourceProjectID
	if err := PersistGeneratePayload(replayPayload); err != nil {
		return dto.PodcastAudioGeneratePayload{}, err
	}
	return replayPayload, nil
}

func BuildGeneratePayloadFromSavedAndCurrent(saved, current dto.PodcastAudioGeneratePayload) (dto.PodcastAudioGeneratePayload, error) {
	targetProjectID := strings.TrimSpace(current.ProjectID)
	if targetProjectID == "" {
		return dto.PodcastAudioGeneratePayload{}, fmt.Errorf("project_id is required")
	}

	lang := strings.ToLower(strings.TrimSpace(saved.Lang))
	currentLang := strings.ToLower(strings.TrimSpace(current.Lang))
	if currentLang != "" && lang != "" && currentLang != lang {
		return dto.PodcastAudioGeneratePayload{}, fmt.Errorf("lang mismatch for replay requested=%s project=%s", currentLang, lang)
	}
	if lang == "" {
		lang = currentLang
	}
	if !validPodcastLanguage(lang) {
		return dto.PodcastAudioGeneratePayload{}, fmt.Errorf("lang must be zh or ja")
	}

	return dto.PodcastAudioGeneratePayload{
		ProjectID:       targetProjectID,
		SourceProjectID: strings.TrimSpace(current.SourceProjectID),
		Lang:            lang,
		ContentProfile:  firstNonEmpty(saved.ContentProfile, current.ContentProfile),
		TTSType:         firstPositive(saved.TTSType, current.TTSType, 1),
		Seed:            firstPositive(saved.Seed, current.Seed),
		RunMode:         firstPositive(current.RunMode, saved.RunMode),
		OnlyCurrentStep: normalizeOnlyCurrentStep(current.OnlyCurrentStep),
		BlockNums:       firstPositiveInts(current.BlockNums, saved.BlockNums),
		Title:           firstNonEmpty(current.Title, saved.Title),
		ScriptFilename:  firstNonEmpty(saved.ScriptFilename, current.ScriptFilename),
		BgImgFilenames:  firstNonEmptyStrings(current.BgImgFilenames, saved.BgImgFilenames),
		TargetPlatform:  firstNonEmpty(current.TargetPlatform, saved.TargetPlatform),
		AspectRatio:     firstNonEmpty(current.AspectRatio, saved.AspectRatio),
		Resolution:      firstNonEmpty(current.Resolution, saved.Resolution),
		DesignStyle:     normalizePodcastDesignStyle(firstPositive(current.DesignStyle, saved.DesignStyle, 1)),
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
		SourceProjectID: strings.TrimSpace(generate.SourceProjectID),
		Lang:            lang,
		RunMode:         generate.RunMode,
		OnlyCurrentStep: normalizeOnlyCurrentStep(generate.OnlyCurrentStep),
		Title:           strings.TrimSpace(generate.Title),
		BgImgFilenames:  backgrounds,
		TargetPlatform:  strings.TrimSpace(generate.TargetPlatform),
		AspectRatio:     strings.TrimSpace(generate.AspectRatio),
		Resolution:      strings.TrimSpace(generate.Resolution),
		DesignStyle:     normalizePodcastDesignStyle(generate.DesignStyle),
	}, nil
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

func normalizeOnlyCurrentStep(value int) int {
	if value == 1 {
		return 1
	}
	return 0
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

func firstPositiveInts(values ...[]int) []int {
	for _, group := range values {
		cleaned := compactPositiveInts(group)
		if len(cleaned) > 0 {
			return cleaned
		}
	}
	return nil
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

func compactNonEmptyStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func compactPositiveInts(values []int) []int {
	seen := make(map[int]struct{})
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
