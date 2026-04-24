package replay

import (
	"encoding/json"
	"fmt"
	"strings"

	"worker/internal/workspace"
	dto "worker/services/practical/model"
)

func DecodeGeneratePayload(raw map[string]interface{}) (dto.PracticalAudioGeneratePayload, error) {
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

func PersistGeneratePayload(payload dto.PracticalAudioGeneratePayload) error {
	return workspace.WriteRequestPayload(payload.ProjectID, payload)
}

func LoadGeneratePayload(projectID string) (dto.PracticalAudioGeneratePayload, error) {
	if strings.TrimSpace(projectID) == "" {
		return dto.PracticalAudioGeneratePayload{}, fmt.Errorf("project_id is required")
	}
	rawPayload, err := workspace.LoadRequestPayloadMap(projectID)
	if err != nil {
		return dto.PracticalAudioGeneratePayload{}, err
	}
	payload, err := DecodeGeneratePayload(rawPayload)
	if err != nil {
		return dto.PracticalAudioGeneratePayload{}, err
	}
	if strings.TrimSpace(payload.ProjectID) == "" {
		payload.ProjectID = strings.TrimSpace(projectID)
	}
	return payload, nil
}

func ResolveSourceProjectID(current dto.PracticalAudioGeneratePayload) (string, error) {
	return ResolveReplaySourceProjectID(current.ProjectID, current.SourceProjectID)
}

func ResolveReplaySourceProjectID(projectID, sourceProjectID string) (string, error) {
	if sourceProjectID = strings.TrimSpace(sourceProjectID); sourceProjectID != "" {
		return sourceProjectID, nil
	}
	return workspace.ReplaySourceProjectID(projectID)
}

func EnsureReplayProjectDirForProject(projectID, sourceProjectID string) error {
	resolvedSourceProjectID, err := ResolveReplaySourceProjectID(projectID, sourceProjectID)
	if err != nil {
		return err
	}
	return workspace.EnsureReplayProjectDir(resolvedSourceProjectID, strings.TrimSpace(projectID))
}

func PrepareReplayPayload(current dto.PracticalAudioGeneratePayload, requestPayloadPatch ...map[string]interface{}) (dto.PracticalAudioGeneratePayload, error) {
	sourceProjectID, err := ResolveSourceProjectID(current)
	if err != nil {
		return dto.PracticalAudioGeneratePayload{}, err
	}
	if err := EnsureReplayProjectDirForProject(current.ProjectID, sourceProjectID); err != nil {
		return dto.PracticalAudioGeneratePayload{}, err
	}

	savedPayload, err := LoadGeneratePayload(sourceProjectID)
	if err != nil {
		return dto.PracticalAudioGeneratePayload{}, err
	}
	replayPayload, err := BuildGeneratePayloadFromSavedAndCurrent(savedPayload, current)
	if err != nil {
		return dto.PracticalAudioGeneratePayload{}, err
	}
	replayPayload.SourceProjectID = sourceProjectID
	if len(requestPayloadPatch) > 0 && requestPayloadPatch[0] != nil {
		if err := persistReplayRequestPayloadPatch(replayPayload.ProjectID, requestPayloadPatch[0]); err != nil {
			return dto.PracticalAudioGeneratePayload{}, err
		}
		return replayPayload, nil
	}
	if err := PersistGeneratePayload(replayPayload); err != nil {
		return dto.PracticalAudioGeneratePayload{}, err
	}
	return replayPayload, nil
}

func PrepareGeneratePayload(current dto.PracticalAudioGeneratePayload, requestPayloadPatch ...map[string]interface{}) (dto.PracticalAudioGeneratePayload, error) {
	return PrepareReplayPayload(current, requestPayloadPatch...)
}

func persistReplayRequestPayloadPatch(projectID string, patch map[string]interface{}) error {
	basePayload, err := workspace.LoadRequestPayloadMap(projectID)
	if err != nil {
		return err
	}
	merged := mergePayloadPatch(basePayload, patch)
	return workspace.WriteRequestPayload(projectID, merged)
}

func mergePayloadPatch(base, patch map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(base)+len(patch))
	for key, value := range base {
		out[key] = value
	}
	for key, value := range patch {
		out[key] = value
	}
	return out
}

func BuildGeneratePayloadFromSavedAndCurrent(saved, current dto.PracticalAudioGeneratePayload) (dto.PracticalAudioGeneratePayload, error) {
	targetProjectID := strings.TrimSpace(current.ProjectID)
	if targetProjectID == "" {
		return dto.PracticalAudioGeneratePayload{}, fmt.Errorf("project_id is required")
	}

	lang := strings.ToLower(strings.TrimSpace(saved.Lang))
	currentLang := strings.ToLower(strings.TrimSpace(current.Lang))
	if currentLang != "" && lang != "" && currentLang != lang {
		return dto.PracticalAudioGeneratePayload{}, fmt.Errorf("lang mismatch for replay requested=%s project=%s", currentLang, lang)
	}
	if lang == "" {
		lang = currentLang
	}
	if !validPracticalLanguage(lang) {
		return dto.PracticalAudioGeneratePayload{}, fmt.Errorf("lang must be zh or ja")
	}

	return dto.PracticalAudioGeneratePayload{
		ProjectID:           targetProjectID,
		SourceProjectID:     strings.TrimSpace(current.SourceProjectID),
		Lang:                lang,
		TTSType:             normalizePracticalTTSType(firstPositive(current.TTSType, saved.TTSType, 1)),
		RunMode:             firstPositive(current.RunMode, saved.RunMode),
		SpecifyTasks:        firstNonEmptyStrings(current.SpecifyTasks, saved.SpecifyTasks),
		BlockNums:           firstPositiveInts(current.BlockNums, saved.BlockNums),
		ScriptFilename:      firstNonEmpty(saved.ScriptFilename, current.ScriptFilename),
		BgImgFilenames:      firstNonEmptyStrings(current.BgImgFilenames, saved.BgImgFilenames),
		BlockBgImgFilenames: firstNonEmptyStrings(current.BlockBgImgFilenames, saved.BlockBgImgFilenames),
		Resolution:          firstNonEmpty(current.Resolution, saved.Resolution),
		AspectRatio:         firstNonEmpty(current.AspectRatio, saved.AspectRatio),
		DesignType:          normalizePracticalDesignType(firstPositive(current.DesignType, saved.DesignType, 1)),
	}, nil
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

func compactNonEmptyStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func validPracticalLanguage(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "zh", "ja":
		return true
	default:
		return false
	}
}

func normalizePracticalDesignType(value int) int {
	if value == 2 {
		return 2
	}
	return 1
}

func normalizePracticalTTSType(value int) int {
	if value == 1 {
		return 1
	}
	return 1
}
