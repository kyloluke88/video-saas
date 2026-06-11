package client

import (
	"fmt"
	"strings"
)

type practicalStage string

const (
	practicalStageGenerate practicalStage = "generate"
	practicalStageAlign    practicalStage = "align"
	practicalStageImages   practicalStage = "images"
	practicalStageRender   practicalStage = "render"
	practicalStagePersist  practicalStage = "persist"
)

type practicalStagePlan struct {
	Start practicalStage
	Stop  practicalStage
}

var practicalStageOrder = []practicalStage{
	practicalStageGenerate,
	practicalStageAlign,
	practicalStageImages,
	practicalStageRender,
	practicalStagePersist,
}

var practicalElevenStageOrder = []practicalStage{
	practicalStageGenerate,
	practicalStageImages,
	practicalStageRender,
	practicalStagePersist,
}

var practicalStageTaskTypes = map[practicalStage]string{
	practicalStageGenerate: "practical.audio.generate.v1",
	practicalStageAlign:    "practical.audio.align.v1",
	practicalStageImages:   "practical.image.generate.v1",
	practicalStageRender:   "practical.compose.render.v1",
	practicalStagePersist:  "practical.page.persist.v1",
}

func normalizePracticalRunMode(value int) int {
	if value == 1 {
		return 1
	}
	return 0
}

func normalizePracticalTTSType(value int) int {
	if value == 2 {
		return 2
	}
	return 1
}

func practicalStageOrderForTTSType(ttsType int) []practicalStage {
	if normalizePracticalTTSType(ttsType) == 2 {
		return practicalElevenStageOrder
	}
	return practicalStageOrder
}

func practicalStageForTaskType(taskType string) string {
	switch strings.TrimSpace(taskType) {
	case "practical.audio.generate.v1":
		return string(practicalStageGenerate)
	case "practical.audio.align.v1":
		return string(practicalStageAlign)
	case "practical.image.generate.v1":
		return string(practicalStageImages)
	case "practical.compose.render.v1":
		return string(practicalStageRender)
	case "practical.page.persist.v1":
		return string(practicalStagePersist)
	default:
		return ""
	}
}

func normalizePracticalDesignType(value int) int {
	if value == 2 {
		return 2
	}
	return 1
}

func parsePracticalStage(value string) (practicalStage, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case string(practicalStageGenerate):
		return practicalStageGenerate, true
	case string(practicalStageAlign):
		return practicalStageAlign, true
	case string(practicalStageImages):
		return practicalStageImages, true
	case string(practicalStageRender):
		return practicalStageRender, true
	case string(practicalStagePersist):
		return practicalStagePersist, true
	default:
		return "", false
	}
}

func resolvePracticalStagePlan(ttsType int, runMode int, startFrom string, stopAt string) (practicalStagePlan, error) {
	normalizedRunMode := normalizePracticalRunMode(runMode)
	normalizedTTSType := normalizePracticalTTSType(ttsType)
	start := strings.TrimSpace(startFrom)
	if normalizedRunMode == 0 {
		if start == "" {
			start = string(practicalStageGenerate)
		}
		if start != string(practicalStageGenerate) {
			return practicalStagePlan{}, fmt.Errorf("start_from must be generate when run_mode is 0")
		}
	} else if start == "" {
		return practicalStagePlan{}, fmt.Errorf("start_from is required when run_mode is 1")
	}

	startStage, ok := parsePracticalStage(start)
	if !ok {
		return practicalStagePlan{}, fmt.Errorf("unsupported start_from value: %s", start)
	}
	order := practicalStageOrderForTTSType(normalizedTTSType)
	startIndex := practicalStageIndex(order, startStage)
	if startIndex < 0 {
		return practicalStagePlan{}, fmt.Errorf("start_from stage %q is not supported for tts_type=%d", startStage, normalizedTTSType)
	}

	plan := practicalStagePlan{Start: startStage}
	if strings.TrimSpace(stopAt) == "" {
		return plan, nil
	}

	stopStage, ok := parsePracticalStage(stopAt)
	if !ok {
		return practicalStagePlan{}, fmt.Errorf("unsupported stop_at value: %s", strings.TrimSpace(stopAt))
	}
	stopIndex := practicalStageIndex(order, stopStage)
	if stopIndex < 0 {
		return practicalStagePlan{}, fmt.Errorf("stop_at stage %q is not supported for tts_type=%d", stopStage, normalizedTTSType)
	}
	if stopIndex < startIndex {
		return practicalStagePlan{}, fmt.Errorf("stop_at %q cannot be earlier than start_from %q", stopStage, startStage)
	}
	plan.Stop = stopStage
	return plan, nil
}

func practicalTaskTypeForStage(ttsType int, stage practicalStage) (string, error) {
	stage = practicalStage(strings.ToLower(strings.TrimSpace(string(stage))))
	if practicalStageIndex(practicalStageOrderForTTSType(ttsType), stage) < 0 {
		return "", fmt.Errorf("stage %q is not supported for tts_type=%d", stage, normalizePracticalTTSType(ttsType))
	}
	taskType, ok := practicalStageTaskTypes[stage]
	if !ok {
		return "", fmt.Errorf("unsupported practical stage %q", stage)
	}
	return taskType, nil
}

func practicalTaskTypeForPlan(ttsType int, plan practicalStagePlan) (string, error) {
	return practicalTaskTypeForStage(ttsType, plan.Start)
}

func practicalTerminalStage(ttsType int) practicalStage {
	order := practicalStageOrderForTTSType(ttsType)
	return order[len(order)-1]
}

func practicalStageIndex(order []practicalStage, stage practicalStage) int {
	for idx, item := range order {
		if item == stage {
			return idx
		}
	}
	return -1
}
