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

func normalizePodcastSpecifyTasks(ttsType int, values []string) ([]string, error) {
	ordered := podcastStageOrder(ttsType)
	if len(values) == 0 {
		return nil, nil
	}

	allowed := make(map[podcastStage]struct{}, len(ordered))
	for _, stage := range ordered {
		allowed[stage] = struct{}{}
	}

	seen := make(map[podcastStage]struct{}, len(values))
	for _, raw := range values {
		if strings.TrimSpace(raw) == "" {
			continue
		}
		stage, ok := parsePodcastStage(raw)
		if !ok {
			return nil, fmt.Errorf("unsupported specify_tasks value: %s", strings.TrimSpace(raw))
		}
		if _, ok := allowed[stage]; !ok {
			return nil, fmt.Errorf("specify_tasks stage %q is not supported for tts_type=%d", stage, normalizePodcastTTSType(ttsType))
		}
		if _, exists := seen[stage]; exists {
			return nil, fmt.Errorf("specify_tasks contains duplicate stage %q", stage)
		}
		seen[stage] = struct{}{}
	}

	out := make([]string, 0, len(seen))
	for _, stage := range ordered {
		if _, ok := seen[stage]; ok {
			out = append(out, string(stage))
		}
	}
	return out, nil
}

func podcastInitialStage(ttsType int, runMode int, specifyTasks []string) (podcastStage, error) {
	if normalizePodcastRunMode(runMode) == 0 {
		return podcastStageGenerate, nil
	}

	normalized, err := normalizePodcastSpecifyTasks(ttsType, specifyTasks)
	if err != nil {
		return "", err
	}
	if len(normalized) == 0 {
		return "", fmt.Errorf("specify_tasks is required when run_mode is 1")
	}
	stage, ok := parsePodcastStage(normalized[0])
	if !ok {
		return "", fmt.Errorf("unsupported specify_tasks value: %s", normalized[0])
	}
	return stage, nil
}

func podcastTaskTypeForStage(ttsType int, stage podcastStage) (string, error) {
	stage = podcastStage(strings.ToLower(strings.TrimSpace(string(stage))))
	allowed := make(map[podcastStage]struct{}, len(podcastStageOrder(ttsType)))
	for _, item := range podcastStageOrder(ttsType) {
		allowed[item] = struct{}{}
	}
	if _, ok := allowed[stage]; !ok {
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

func podcastTaskTypeForInitialStage(ttsType int, runMode int, specifyTasks []string) (string, error) {
	stage, err := podcastInitialStage(ttsType, runMode, specifyTasks)
	if err != nil {
		return "", err
	}
	return podcastTaskTypeForStage(ttsType, stage)
}

func podcastSpecifiesStage(values []string, target podcastStage) bool {
	for _, raw := range values {
		stage, ok := parsePodcastStage(raw)
		if !ok {
			continue
		}
		if stage == target {
			return true
		}
	}
	return false
}
