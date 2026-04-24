package replay

import (
	"fmt"
	"strings"
)

type PracticalStage string

const (
	PracticalStageGenerate PracticalStage = "generate"
	PracticalStageAlign    PracticalStage = "align"
	PracticalStageRender   PracticalStage = "render"
	PracticalStageFinalize PracticalStage = "finalize"
	PracticalStagePersist  PracticalStage = "persist"
)

var practicalStageOrder = []PracticalStage{
	PracticalStageGenerate,
	PracticalStageAlign,
	PracticalStageRender,
	PracticalStageFinalize,
	PracticalStagePersist,
}

var practicalStageTaskTypes = map[PracticalStage]string{
	PracticalStageGenerate: "practical.audio.generate.v1",
	PracticalStageAlign:    "practical.audio.align.v1",
	PracticalStageRender:   "practical.compose.render.v1",
	PracticalStageFinalize: "practical.compose.finalize.v1",
	PracticalStagePersist:  "practical.page.persist.v1",
}

func parsePracticalStage(value string) (PracticalStage, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case string(PracticalStageGenerate):
		return PracticalStageGenerate, true
	case string(PracticalStageAlign):
		return PracticalStageAlign, true
	case string(PracticalStageRender):
		return PracticalStageRender, true
	case string(PracticalStageFinalize):
		return PracticalStageFinalize, true
	case string(PracticalStagePersist):
		return PracticalStagePersist, true
	default:
		return "", false
	}
}

func PracticalStageForTaskType(taskType string) (PracticalStage, bool) {
	switch strings.TrimSpace(taskType) {
	case "practical.audio.generate.v1":
		return PracticalStageGenerate, true
	case "practical.audio.align.v1":
		return PracticalStageAlign, true
	case "practical.compose.render.v1":
		return PracticalStageRender, true
	case "practical.compose.finalize.v1":
		return PracticalStageFinalize, true
	case "practical.page.persist.v1":
		return PracticalStagePersist, true
	default:
		return "", false
	}
}

func PracticalTaskTypeForStage(stage PracticalStage) (string, error) {
	stage = PracticalStage(strings.ToLower(strings.TrimSpace(string(stage))))
	if _, ok := practicalTaskTypeMap()[stage]; !ok {
		return "", fmt.Errorf("unsupported practical stage %q", stage)
	}
	taskType, ok := practicalStageTaskTypes[stage]
	if !ok {
		return "", fmt.Errorf("unsupported practical stage %q", stage)
	}
	return taskType, nil
}

func NormalizeSpecifyTasks(values []string) ([]string, error) {
	if len(values) == 0 {
		return nil, nil
	}

	seen := make(map[PracticalStage]struct{}, len(values))
	for _, raw := range values {
		if strings.TrimSpace(raw) == "" {
			continue
		}
		stage, ok := parsePracticalStage(raw)
		if !ok {
			return nil, fmt.Errorf("unsupported specify_tasks value: %s", strings.TrimSpace(raw))
		}
		if _, exists := seen[stage]; exists {
			return nil, fmt.Errorf("specify_tasks contains duplicate stage %q", stage)
		}
		seen[stage] = struct{}{}
	}

	out := make([]string, 0, len(seen))
	for _, stage := range practicalStageOrder {
		if _, ok := seen[stage]; ok {
			out = append(out, string(stage))
		}
	}
	return out, nil
}

func ValidateSpecifyTasks(runMode int, values []string) ([]string, error) {
	normalized, err := NormalizeSpecifyTasks(values)
	if err != nil {
		return nil, err
	}
	if runMode == 1 && len(normalized) == 0 {
		return nil, fmt.Errorf("specify_tasks is required when run_mode=1")
	}
	return normalized, nil
}

func NextPracticalStage(current string, specifyTasks []string) (string, bool, error) {
	currentStage, ok := parsePracticalStage(current)
	if !ok {
		return "", false, fmt.Errorf("unsupported practical stage: %s", strings.TrimSpace(current))
	}
	if len(specifyTasks) == 0 {
		return defaultNextPracticalStage(currentStage)
	}

	normalized, err := NormalizeSpecifyTasks(specifyTasks)
	if err != nil {
		return "", false, err
	}
	if len(normalized) == 0 {
		return defaultNextPracticalStage(currentStage)
	}

	currentIndex := stageIndex(practicalStageOrder, currentStage)
	if currentIndex < 0 {
		return "", false, fmt.Errorf("unsupported practical stage: %s", currentStage)
	}
	for _, raw := range normalized {
		stage, _ := parsePracticalStage(raw)
		if idx := stageIndex(practicalStageOrder, stage); idx > currentIndex {
			return string(stage), true, nil
		}
	}
	return "", false, nil
}

func defaultNextPracticalStage(current PracticalStage) (string, bool, error) {
	idx := stageIndex(practicalStageOrder, current)
	if idx < 0 {
		return "", false, fmt.Errorf("unsupported practical stage: %s", current)
	}
	if idx+1 >= len(practicalStageOrder) {
		return "", false, nil
	}
	return string(practicalStageOrder[idx+1]), true, nil
}

func practicalTaskTypeMap() map[PracticalStage]string {
	return practicalStageTaskTypes
}

func stageIndex(order []PracticalStage, stage PracticalStage) int {
	for idx, item := range order {
		if item == stage {
			return idx
		}
	}
	return -1
}
