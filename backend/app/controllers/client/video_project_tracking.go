package client

import (
	"encoding/json"
	"strings"
	"time"

	"api/app/models/content"
	"api/pkg/database"
	"api/pkg/logger"

	"go.uber.org/zap"
)

func trackProject(
	projectID string,
	contentType string,
	runMode *int,
	currentStage string,
	targetTaskType string,
	requestPayload map[string]interface{},
) {
	if database.DB == nil {
		return
	}

	now := time.Now().UTC()
	payloadJSON, err := json.Marshal(requestPayload)
	if err != nil {
		logger.Warn("project tracking marshal failed", zap.Error(err), zap.String("project_id", projectID))
		return
	}

	project := content.Project{
		ProjectID:       projectID,
		ContentType:     contentType,
		RunMode:         runMode,
		RetryNum:        0,
		Status:          content.ProjectStatusQueued,
		CurrentStage:    currentStage,
		CurrentTaskType: targetTaskType,
		LastError:       "",
		Payload:         payloadJSON,
	}

	if err := content.UpsertProject(project, now); err != nil {
		logger.Warn("project tracking upsert failed", zap.Error(err), zap.String("project_id", projectID))
	}
}

func trackPodcastProject(
	projectID string,
	runMode int,
	targetTaskType string,
	requestPayload map[string]interface{},
) {
	runModeValue := runMode
	payloadForTracking := buildTrackedPodcastPayload(runMode, requestPayload)
	trackProject(projectID, "podcast", &runModeValue, podcastTriggerStage(runMode), targetTaskType, payloadForTracking)
}

func trackIdiomProject(projectID string, targetTaskType string, requestPayload map[string]interface{}) {
	runModeValue := 0
	trackProject(projectID, "idiom", &runModeValue, "plan", targetTaskType, requestPayload)
}

func markProjectRequestFailed(projectID string, taskType string, err error) {
	if projectID == "" || database.DB == nil {
		return
	}
	errorMessage := ""
	if err != nil {
		errorMessage = err.Error()
	}
	now := time.Now().UTC()
	if updateErr := content.UpdateProjectByProjectID(projectID, map[string]interface{}{
		"status":            content.ProjectStatusError,
		"current_task_type": taskType,
		"last_error":        errorMessage,
		"finished_at":       &now,
		"updated_at":        now,
	}); updateErr != nil {
		logger.Warn("project mark failed failed", zap.Error(updateErr), zap.String("project_id", projectID))
	}
}

func markPodcastProjectRequestFailed(projectID string, taskType string, err error) {
	markProjectRequestFailed(projectID, taskType, err)
}

func podcastTriggerStage(runMode int) string {
	switch runMode {
	case 2:
		return "compose"
	case 3:
		return "script_persist"
	case 4:
		return "audio_align"
	default:
		return "audio_generate"
	}
}

func buildTrackedPodcastPayload(runMode int, requestPayload map[string]interface{}) map[string]interface{} {
	patch := cloneStringAnyMap(requestPayload)
	if runMode == 0 {
		return patch
	}
	sourceProjectID := strings.TrimSpace(anyString(requestPayload["source_project_id"]))
	if sourceProjectID == "" || database.DB == nil {
		return patch
	}

	sourceProject, err := content.FindProjectByProjectID(sourceProjectID)
	if err != nil {
		logger.Warn("load source project payload failed", zap.Error(err), zap.String("source_project_id", sourceProjectID))
		return patch
	}
	if len(sourceProject.Payload) == 0 {
		return patch
	}

	base := make(map[string]interface{})
	if err := json.Unmarshal(sourceProject.Payload, &base); err != nil {
		logger.Warn("decode source project payload failed", zap.Error(err), zap.String("source_project_id", sourceProjectID))
		return patch
	}
	return mergeStringAnyMap(base, patch)
}

func cloneStringAnyMap(src map[string]interface{}) map[string]interface{} {
	if len(src) == 0 {
		return map[string]interface{}{}
	}
	out := make(map[string]interface{}, len(src))
	for key, value := range src {
		out[key] = value
	}
	return out
}

func mergeStringAnyMap(base, patch map[string]interface{}) map[string]interface{} {
	out := cloneStringAnyMap(base)
	for key, value := range patch {
		out[key] = value
	}
	return out
}
