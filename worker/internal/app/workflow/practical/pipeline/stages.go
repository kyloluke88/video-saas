package pipeline

import (
	"fmt"
	"strings"
)

type Stage string

const (
	StageGenerate Stage = "generate"
	StageAlign    Stage = "align"
	StageImages   Stage = "images"
	StageRender   Stage = "render"
	StagePersist  Stage = "persist"
)

var stageOrder = []Stage{
	StageGenerate,
	StageAlign,
	StageImages,
	StageRender,
	StagePersist,
}

var stageTaskTypes = map[Stage]string{
	StageGenerate: "practical.audio.generate.v1",
	StageAlign:    "practical.audio.align.v1",
	StageImages:   "practical.image.generate.v1",
	StageRender:   "practical.compose.render.v1",
	StagePersist:  "practical.page.persist.v1",
}

func NormalizeTTSType(value int) int {
	return 1
}

func StageOrder(ttsType int) []Stage {
	return stageOrder
}

func ParseStage(value string) (Stage, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case string(StageGenerate):
		return StageGenerate, true
	case string(StageAlign):
		return StageAlign, true
	case string(StageImages):
		return StageImages, true
	case string(StageRender):
		return StageRender, true
	case string(StagePersist):
		return StagePersist, true
	default:
		return "", false
	}
}

func StageForTaskType(taskType string) (Stage, bool) {
	switch strings.TrimSpace(taskType) {
	case "practical.audio.generate.v1":
		return StageGenerate, true
	case "practical.audio.align.v1":
		return StageAlign, true
	case "practical.image.generate.v1":
		return StageImages, true
	case "practical.compose.render.v1":
		return StageRender, true
	case "practical.page.persist.v1":
		return StagePersist, true
	default:
		return "", false
	}
}

func TaskTypeForStage(ttsType int, stage Stage) (string, error) {
	stage = Stage(strings.ToLower(strings.TrimSpace(string(stage))))
	if StageIndex(StageOrder(ttsType), stage) < 0 {
		return "", fmt.Errorf("stage %q is not supported for tts_type=%d", stage, NormalizeTTSType(ttsType))
	}
	taskType, ok := stageTaskTypes[stage]
	if !ok {
		return "", fmt.Errorf("unsupported practical stage %q", stage)
	}
	return taskType, nil
}

func TerminalStage(ttsType int) Stage {
	order := StageOrder(ttsType)
	return order[len(order)-1]
}

func ValidateRange(ttsType int, runMode int, startFrom string, stopAt string) (Stage, Stage, error) {
	normalizedRunMode := runMode
	if normalizedRunMode != 1 {
		normalizedRunMode = 0
	}

	start := strings.TrimSpace(startFrom)
	if normalizedRunMode == 0 {
		if start == "" {
			start = string(StageGenerate)
		}
		if start != string(StageGenerate) {
			return "", "", fmt.Errorf("start_from must be generate when run_mode=0")
		}
	} else if start == "" {
		return "", "", fmt.Errorf("start_from is required when run_mode=1")
	}

	startStage, ok := ParseStage(start)
	if !ok {
		return "", "", fmt.Errorf("unsupported start_from value: %s", start)
	}
	order := StageOrder(ttsType)
	startIndex := StageIndex(order, startStage)
	if startIndex < 0 {
		return "", "", fmt.Errorf("start_from stage %q is not supported for tts_type=%d", startStage, NormalizeTTSType(ttsType))
	}

	if strings.TrimSpace(stopAt) == "" {
		return startStage, "", nil
	}

	stopStage, ok := ParseStage(stopAt)
	if !ok {
		return "", "", fmt.Errorf("unsupported stop_at value: %s", strings.TrimSpace(stopAt))
	}
	stopIndex := StageIndex(order, stopStage)
	if stopIndex < 0 {
		return "", "", fmt.Errorf("stop_at stage %q is not supported for tts_type=%d", stopStage, NormalizeTTSType(ttsType))
	}
	if stopIndex < startIndex {
		return "", "", fmt.Errorf("stop_at %q cannot be earlier than start_from %q", stopStage, startStage)
	}
	return startStage, stopStage, nil
}

func NextStage(ttsType int, current string, stopAt string) (string, bool, error) {
	currentStage, ok := ParseStage(current)
	if !ok {
		return "", false, fmt.Errorf("unsupported practical stage: %s", strings.TrimSpace(current))
	}
	order := StageOrder(ttsType)
	currentIndex := StageIndex(order, currentStage)
	if currentIndex < 0 {
		return "", false, fmt.Errorf("unsupported practical stage: %s", currentStage)
	}

	if strings.TrimSpace(stopAt) != "" {
		stopStage, ok := ParseStage(stopAt)
		if !ok {
			return "", false, fmt.Errorf("unsupported stop_at value: %s", strings.TrimSpace(stopAt))
		}
		stopIndex := StageIndex(order, stopStage)
		if stopIndex < 0 {
			return "", false, fmt.Errorf("stop_at stage %q is not supported for tts_type=%d", stopStage, NormalizeTTSType(ttsType))
		}
		if currentIndex >= stopIndex {
			return "", false, nil
		}
	}

	if currentIndex+1 >= len(order) {
		return "", false, nil
	}
	return string(order[currentIndex+1]), true, nil
}

func IsFinalStage(ttsType int, current string) bool {
	stage, ok := ParseStage(current)
	if !ok {
		return false
	}
	return stage == TerminalStage(ttsType)
}

func StageIndex(order []Stage, stage Stage) int {
	for idx, item := range order {
		if item == stage {
			return idx
		}
	}
	return -1
}
