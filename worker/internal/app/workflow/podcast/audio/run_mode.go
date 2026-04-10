package podcast_audio

import (
	"fmt"
	"strings"

	dto "worker/services/podcast/model"
)

func normalizePodcastRunMode(value int) int {
	switch value {
	case 1, 2, 4:
		return value
	default:
		return 0
	}
}

func buildReplayGeneratePayloadFromSavedAndCurrent(saved, current dto.PodcastAudioGeneratePayload) (dto.PodcastAudioGeneratePayload, error) {
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
		ProjectID:      targetProjectID,
		Lang:           lang,
		ContentProfile: firstNonEmpty(saved.ContentProfile, current.ContentProfile),
		TTSType:        firstPositive(saved.TTSType, current.TTSType, 1),
		Seed:           firstPositive(saved.Seed, current.Seed),
		RunMode:        firstPositive(current.RunMode, saved.RunMode),
		BlockNums:      compactPositiveInts(current.BlockNums),
		Title:          firstNonEmpty(current.Title, saved.Title),
		ScriptFilename: firstNonEmpty(saved.ScriptFilename, current.ScriptFilename),
		BgImgFilenames: firstNonEmptyStrings(current.BgImgFilenames, saved.BgImgFilenames),
		TargetPlatform: firstNonEmpty(current.TargetPlatform, saved.TargetPlatform),
		AspectRatio:    firstNonEmpty(current.AspectRatio, saved.AspectRatio),
		Resolution:     firstNonEmpty(current.Resolution, saved.Resolution),
		DesignStyle:    normalizePodcastDesignStyle(firstPositive(current.DesignStyle, saved.DesignStyle, 1)),
	}, nil
}

func buildComposePayloadFromGenerate(generate dto.PodcastAudioGeneratePayload) (dto.PodcastComposePayload, error) {
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
		ProjectID:      projectID,
		Lang:           lang,
		Title:          strings.TrimSpace(generate.Title),
		BgImgFilenames: backgrounds,
		TargetPlatform: strings.TrimSpace(generate.TargetPlatform),
		AspectRatio:    strings.TrimSpace(generate.AspectRatio),
		Resolution:     strings.TrimSpace(generate.Resolution),
		DesignStyle:    normalizePodcastDesignStyle(generate.DesignStyle),
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

func BuildReplayPayloadForPersist(current dto.PodcastAudioGeneratePayload) (dto.PodcastAudioGeneratePayload, error) {
	sourceProjectID, err := resolveReplaySourceProjectID(current)
	if err != nil {
		return dto.PodcastAudioGeneratePayload{}, err
	}
	saved, err := loadPersistedGeneratePayload(sourceProjectID)
	if err != nil {
		return dto.PodcastAudioGeneratePayload{}, err
	}
	return buildReplayGeneratePayloadFromSavedAndCurrent(saved, current)
}
