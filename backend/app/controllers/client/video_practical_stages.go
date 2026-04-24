package client

import (
	"fmt"
	"strings"
)

type practicalStage string

const (
	practicalStageGenerate practicalStage = "generate"
	practicalStageAlign    practicalStage = "align"
	practicalStageRender   practicalStage = "render"
	practicalStageFinalize practicalStage = "finalize"
	practicalStagePersist  practicalStage = "persist"
)

var practicalStageOrder = []practicalStage{
	practicalStageGenerate,
	practicalStageAlign,
	practicalStageRender,
	practicalStageFinalize,
	practicalStagePersist,
}

var practicalStageTaskTypes = map[practicalStage]string{
	practicalStageGenerate: "practical.audio.generate.v1",
	practicalStageAlign:    "practical.audio.align.v1",
	practicalStageRender:   "practical.compose.render.v1",
	practicalStageFinalize: "practical.compose.finalize.v1",
	practicalStagePersist:  "practical.page.persist.v1",
}

func normalizePracticalRunMode(value int) int {
	if value == 1 {
		return 1
	}
	return 0
}

func practicalStageForTaskType(taskType string) string {
	switch strings.TrimSpace(taskType) {
	case "practical.audio.generate.v1":
		return string(practicalStageGenerate)
	case "practical.audio.align.v1":
		return string(practicalStageAlign)
	case "practical.compose.render.v1":
		return string(practicalStageRender)
	case "practical.compose.finalize.v1":
		return string(practicalStageFinalize)
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
	case string(practicalStageRender):
		return practicalStageRender, true
	case string(practicalStageFinalize):
		return practicalStageFinalize, true
	case string(practicalStagePersist):
		return practicalStagePersist, true
	default:
		return "", false
	}
}

func normalizePracticalSpecifyTasks(values []string) ([]string, error) {
	if len(values) == 0 {
		return nil, nil
	}

	seen := make(map[practicalStage]struct{}, len(values))
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

func practicalInitialStage(runMode int, specifyTasks []string) (practicalStage, error) {
	if normalizePracticalRunMode(runMode) == 0 {
		return practicalStageGenerate, nil
	}

	normalized, err := normalizePracticalSpecifyTasks(specifyTasks)
	if err != nil {
		return "", err
	}
	if len(normalized) == 0 {
		return "", fmt.Errorf("specify_tasks is required when run_mode is 1")
	}
	stage, ok := parsePracticalStage(normalized[0])
	if !ok {
		return "", fmt.Errorf("unsupported specify_tasks value: %s", normalized[0])
	}
	return stage, nil
}

func practicalTaskTypeForStage(stage practicalStage) (string, error) {
	stage = practicalStage(strings.ToLower(strings.TrimSpace(string(stage))))
	taskType, ok := practicalStageTaskTypes[stage]
	if !ok {
		return "", fmt.Errorf("unsupported practical stage %q", stage)
	}
	return taskType, nil
}

func practicalTaskTypeForInitialStage(runMode int, specifyTasks []string) (string, error) {
	stage, err := practicalInitialStage(runMode, specifyTasks)
	if err != nil {
		return "", err
	}
	return practicalTaskTypeForStage(stage)
}

func practicalSpecifiesStage(values []string, target practicalStage) bool {
	for _, raw := range values {
		stage, ok := parsePracticalStage(raw)
		if !ok {
			continue
		}
		if stage == target {
			return true
		}
	}
	return false
}
