package client

import (
	"fmt"
	"strings"
)

type podcastStage string

const (
	podcastStageGenerate podcastStage = "generate"
	podcastStageAlign    podcastStage = "align"
	podcastStageRender   podcastStage = "render"
	podcastStageFinalize podcastStage = "finalize"
	podcastStagePersist  podcastStage = "persist"
	podcastStageUpload   podcastStage = "upload"
)

type podcastStagePlan struct {
	Start podcastStage
	Stop  podcastStage
}

var podcastGoogleStageOrder = []podcastStage{
	podcastStageGenerate,
	podcastStageAlign,
	podcastStageRender,
	podcastStageFinalize,
	podcastStagePersist,
	podcastStageUpload,
}

var podcastElevenStageOrder = []podcastStage{
	podcastStageGenerate,
	podcastStageRender,
	podcastStageFinalize,
	podcastStagePersist,
	podcastStageUpload,
}

var podcastStageTaskTypes = map[podcastStage]string{
	podcastStageGenerate: "podcast.audio.generate.v1",
	podcastStageAlign:    "podcast.audio.align.v1",
	podcastStageRender:   "podcast.compose.render.v1",
	podcastStageFinalize: "podcast.compose.finalize.v1",
	podcastStagePersist:  "podcast.page.persist.v1",
	podcastStageUpload:   "upload.v1",
}

func normalizePodcastRunMode(value int) int {
	if value == 1 {
		return 1
	}
	return 0
}

func normalizePodcastTTSType(value int) int {
	if value == 2 {
		return 2
	}
	return 1
}

func podcastStageOrder(ttsType int) []podcastStage {
	if normalizePodcastTTSType(ttsType) == 2 {
		return podcastElevenStageOrder
	}
	return podcastGoogleStageOrder
}

func podcastTerminalStage(ttsType int) podcastStage {
	order := podcastStageOrder(ttsType)
	return order[len(order)-1]
}

func parsePodcastStage(value string) (podcastStage, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case string(podcastStageGenerate):
		return podcastStageGenerate, true
	case string(podcastStageAlign):
		return podcastStageAlign, true
	case string(podcastStageRender):
		return podcastStageRender, true
	case string(podcastStageFinalize):
		return podcastStageFinalize, true
	case string(podcastStagePersist):
		return podcastStagePersist, true
	case string(podcastStageUpload):
		return podcastStageUpload, true
	default:
		return "", false
	}
}

func resolvePodcastStagePlan(ttsType int, runMode int, startFrom string, stopAt string) (podcastStagePlan, error) {
	normalizedRunMode := normalizePodcastRunMode(runMode)
	normalizedTTSType := normalizePodcastTTSType(ttsType)

	start := strings.TrimSpace(startFrom)
	if normalizedRunMode == 0 {
		if start == "" {
			start = string(podcastStageGenerate)
		}
		if start != string(podcastStageGenerate) {
			return podcastStagePlan{}, fmt.Errorf("start_from must be generate when run_mode is 0")
		}
	} else if start == "" {
		return podcastStagePlan{}, fmt.Errorf("start_from is required when run_mode is 1")
	}

	startStage, ok := parsePodcastStage(start)
	if !ok {
		return podcastStagePlan{}, fmt.Errorf("unsupported start_from value: %s", start)
	}

	order := podcastStageOrder(normalizedTTSType)
	startIndex := podcastStageIndex(order, startStage)
	if startIndex < 0 {
		return podcastStagePlan{}, fmt.Errorf("start_from stage %q is not supported for tts_type=%d", startStage, normalizedTTSType)
	}

	plan := podcastStagePlan{Start: startStage}
	if strings.TrimSpace(stopAt) == "" {
		return plan, nil
	}

	stopStage, ok := parsePodcastStage(stopAt)
	if !ok {
		return podcastStagePlan{}, fmt.Errorf("unsupported stop_at value: %s", strings.TrimSpace(stopAt))
	}
	stopIndex := podcastStageIndex(order, stopStage)
	if stopIndex < 0 {
		return podcastStagePlan{}, fmt.Errorf("stop_at stage %q is not supported for tts_type=%d", stopStage, normalizedTTSType)
	}
	if stopIndex < startIndex {
		return podcastStagePlan{}, fmt.Errorf("stop_at %q cannot be earlier than start_from %q", stopStage, startStage)
	}
	plan.Stop = stopStage
	return plan, nil
}

func podcastTaskTypeForStage(ttsType int, stage podcastStage) (string, error) {
	stage = podcastStage(strings.ToLower(strings.TrimSpace(string(stage))))
	if podcastStageIndex(podcastStageOrder(ttsType), stage) < 0 {
		return "", fmt.Errorf("stage %q is not supported for tts_type=%d", stage, normalizePodcastTTSType(ttsType))
	}
	taskType, ok := podcastStageTaskTypes[stage]
	if !ok {
		return "", fmt.Errorf("unsupported podcast stage %q", stage)
	}
	return taskType, nil
}

func podcastStageForTaskType(taskType string) string {
	switch strings.TrimSpace(taskType) {
	case "podcast.audio.generate.v1":
		return string(podcastStageGenerate)
	case "podcast.audio.align.v1":
		return string(podcastStageAlign)
	case "podcast.compose.render.v1":
		return string(podcastStageRender)
	case "podcast.compose.finalize.v1":
		return string(podcastStageFinalize)
	case "podcast.page.persist.v1":
		return string(podcastStagePersist)
	case "upload.v1":
		return string(podcastStageUpload)
	default:
		return ""
	}
}

func podcastTaskTypeForPlan(ttsType int, plan podcastStagePlan) (string, error) {
	return podcastTaskTypeForStage(ttsType, plan.Start)
}

func podcastStageIndex(order []podcastStage, stage podcastStage) int {
	for idx, item := range order {
		if item == stage {
			return idx
		}
	}
	return -1
}
