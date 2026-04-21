package replay

import (
	"fmt"
	"strings"
)

type PodcastStage string

const (
	PodcastStageGenerate PodcastStage = "generate"
	PodcastStageAlign    PodcastStage = "align"
	PodcastStageRender   PodcastStage = "render"
	PodcastStageFinalize PodcastStage = "finalize"
	PodcastStagePersist  PodcastStage = "persist"
	PodcastStageUpload   PodcastStage = "upload"
)

var podcastGoogleStageOrder = []PodcastStage{
	PodcastStageGenerate,
	PodcastStageAlign,
	PodcastStageRender,
	PodcastStageFinalize,
	PodcastStagePersist,
	PodcastStageUpload,
}

var podcastElevenStageOrder = []PodcastStage{
	PodcastStageGenerate,
	PodcastStageRender,
	PodcastStageFinalize,
	PodcastStagePersist,
	PodcastStageUpload,
}

var podcastStageTaskTypes = map[PodcastStage]string{
	PodcastStageGenerate: "podcast.audio.generate.v1",
	PodcastStageAlign:    "podcast.audio.align.v1",
	PodcastStageRender:   "podcast.compose.render.v1",
	PodcastStageFinalize: "podcast.compose.finalize.v1",
	PodcastStagePersist:  "podcast.page.persist.v1",
	PodcastStageUpload:   "upload.v1",
}

func normalizePodcastTTSType(value int) int {
	if value == 2 {
		return 2
	}
	return 1
}

func podcastStageOrder(ttsType int) []PodcastStage {
	if normalizePodcastTTSType(ttsType) == 2 {
		return podcastElevenStageOrder
	}
	return podcastGoogleStageOrder
}

func parsePodcastStage(value string) (PodcastStage, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case string(PodcastStageGenerate):
		return PodcastStageGenerate, true
	case string(PodcastStageAlign):
		return PodcastStageAlign, true
	case string(PodcastStageRender):
		return PodcastStageRender, true
	case string(PodcastStageFinalize):
		return PodcastStageFinalize, true
	case string(PodcastStagePersist):
		return PodcastStagePersist, true
	case string(PodcastStageUpload):
		return PodcastStageUpload, true
	default:
		return "", false
	}
}

func PodcastStageForTaskType(taskType string) (PodcastStage, bool) {
	switch strings.TrimSpace(taskType) {
	case "podcast.audio.generate.v1":
		return PodcastStageGenerate, true
	case "podcast.audio.align.v1":
		return PodcastStageAlign, true
	case "podcast.compose.render.v1":
		return PodcastStageRender, true
	case "podcast.compose.finalize.v1":
		return PodcastStageFinalize, true
	case "podcast.page.persist.v1":
		return PodcastStagePersist, true
	case "upload.v1":
		return PodcastStageUpload, true
	default:
		return "", false
	}
}

func PodcastTaskTypeForStage(ttsType int, stage PodcastStage) (string, error) {
	stage = PodcastStage(strings.ToLower(strings.TrimSpace(string(stage))))
	allowed := make(map[PodcastStage]struct{}, len(podcastStageOrder(ttsType)))
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

func NormalizeSpecifyTasks(ttsType int, values []string) ([]string, error) {
	ordered := podcastStageOrder(ttsType)
	if len(values) == 0 {
		return nil, nil
	}

	allowed := make(map[PodcastStage]struct{}, len(ordered))
	for _, stage := range ordered {
		allowed[stage] = struct{}{}
	}

	seen := make(map[PodcastStage]struct{}, len(values))
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

func ValidateSpecifyTasks(ttsType int, runMode int, values []string) ([]string, error) {
	normalized, err := NormalizeSpecifyTasks(ttsType, values)
	if err != nil {
		return nil, err
	}
	if runMode == 1 && len(normalized) == 0 {
		return nil, fmt.Errorf("specify_tasks is required when run_mode=1")
	}
	return normalized, nil
}

func NextPodcastStage(ttsType int, current string, specifyTasks []string) (string, bool, error) {
	currentStage, ok := parsePodcastStage(current)
	if !ok {
		return "", false, fmt.Errorf("unsupported podcast stage: %s", strings.TrimSpace(current))
	}
	if len(specifyTasks) == 0 {
		next, ok := defaultNextPodcastStage(ttsType, currentStage)
		return next, ok, nil
	}

	normalized, err := NormalizeSpecifyTasks(ttsType, specifyTasks)
	if err != nil {
		return "", false, err
	}
	if len(normalized) == 0 {
		next, ok := defaultNextPodcastStage(ttsType, currentStage)
		return next, ok, nil
	}

	order := podcastStageOrder(ttsType)
	currentIndex := stageIndex(order, currentStage)
	if currentIndex < 0 {
		return "", false, fmt.Errorf("unsupported podcast stage: %s", currentStage)
	}

	for _, raw := range normalized {
		stage, _ := parsePodcastStage(raw)
		if idx := stageIndex(order, stage); idx > currentIndex {
			return string(stage), true, nil
		}
	}
	return "", false, nil
}

func defaultNextPodcastStage(ttsType int, current PodcastStage) (string, bool) {
	order := podcastStageOrder(ttsType)
	currentIndex := stageIndex(order, current)
	if currentIndex < 0 || currentIndex+1 >= len(order) {
		return "", false
	}
	return string(order[currentIndex+1]), true
}

func stageIndex(order []PodcastStage, stage PodcastStage) int {
	for idx, item := range order {
		if item == stage {
			return idx
		}
	}
	return -1
}
