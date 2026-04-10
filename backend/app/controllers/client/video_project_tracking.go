package client

import (
	"encoding/json"
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
	trackProject(projectID, "podcast", &runModeValue, podcastTriggerStage(runMode), targetTaskType, requestPayload)
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
