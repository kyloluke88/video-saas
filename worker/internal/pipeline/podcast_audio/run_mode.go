package podcast_audio

import (
	"fmt"
	"strings"

	"worker/internal/dto"
)

func normalizePodcastRunMode(value int) int {
	switch value {
	case 1, 2:
		return value
	default:
		return 0
	}
}

func buildComposePayloadForRunMode2(saved, current dto.PodcastAudioGeneratePayload) (dto.PodcastComposePayload, error) {
	projectID := strings.TrimSpace(saved.ProjectID)
	if projectID == "" {
		projectID = strings.TrimSpace(current.ProjectID)
	}
	if projectID == "" {
		return dto.PodcastComposePayload{}, fmt.Errorf("project_id is required")
	}

	lang := strings.ToLower(strings.TrimSpace(saved.Lang))
	currentLang := strings.ToLower(strings.TrimSpace(current.Lang))
	if currentLang != "" && lang != "" && currentLang != lang {
		return dto.PodcastComposePayload{}, fmt.Errorf("lang mismatch for run_mode=2 requested=%s project=%s", currentLang, lang)
	}
	if lang == "" {
		lang = currentLang
	}
	if !validPodcastLanguage(lang) {
		return dto.PodcastComposePayload{}, fmt.Errorf("lang must be zh or ja")
	}

	backgrounds := firstNonEmptyStrings(current.BgImgFilenames, saved.BgImgFilenames)
	if len(backgrounds) == 0 {
		return dto.PodcastComposePayload{}, fmt.Errorf("bg_img_filenames is required")
	}

	return dto.PodcastComposePayload{
		ProjectID:      projectID,
		Lang:           lang,
		Title:          firstNonEmpty(current.Title, saved.Title),
		BgImgFilenames: backgrounds,
		TargetPlatform: firstNonEmpty(current.TargetPlatform, saved.TargetPlatform),
		AspectRatio:    firstNonEmpty(current.AspectRatio, saved.AspectRatio),
		Resolution:     firstNonEmpty(current.Resolution, saved.Resolution),
		DesignStyle:    firstPositive(current.DesignStyle, saved.DesignStyle, 1),
	}, nil
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
