package persistence

import (
	"errors"
	"fmt"
	"time"

	contentmodels "worker/internal/persistence/models/content"

	"gorm.io/gorm"
)

type ProjectUpdate struct {
	ProjectID          string
	ContentType        string
	RunMode            *int
	RetryNum           *int
	CurrentStage       string
	CurrentTaskType    string
	TerminatedTaskType string
	Status             int16
	LastError          string
	Payload            []byte
	StartedAt          *time.Time
	FinishedAt         *time.Time
	CancelRequestedAt  *time.Time
	CancelledAt        *time.Time
	CancelSource       string
}

func (s *Store) UpdateProject(input ProjectUpdate) error {
	if input.ProjectID == "" {
		return wrapFatal(errors.New("project_id is required"))
	}

	updates := map[string]interface{}{
		"content_type":         input.ContentType,
		"status":               input.Status,
		"current_stage":        input.CurrentStage,
		"current_task_type":    input.CurrentTaskType,
		"terminated_task_type": input.TerminatedTaskType,
		"last_error":           input.LastError,
	}
	if input.RunMode != nil {
		updates["run_mode"] = *input.RunMode
	}
	if input.RetryNum != nil {
		updates["retry_num"] = *input.RetryNum
	}
	if len(input.Payload) > 0 {
		updates["payload_json"] = input.Payload
	}
	if input.StartedAt != nil {
		updates["started_at"] = gorm.Expr("COALESCE(started_at, ?)", *input.StartedAt)
	}
	if input.FinishedAt != nil {
		updates["finished_at"] = input.FinishedAt
	}
	if input.CancelRequestedAt != nil {
		updates["cancel_requested_at"] = input.CancelRequestedAt
	}
	if input.CancelledAt != nil {
		updates["cancelled_at"] = input.CancelledAt
	}
	if input.CancelSource != "" {
		updates["cancel_source"] = input.CancelSource
	}

	result := s.db.Model(&contentmodels.Project{}).
		Where("project_id = ?", input.ProjectID).
		Updates(updates)
	if result.Error != nil {
		return wrapFatal(result.Error)
	}
	if result.RowsAffected == 0 {
		return wrapFatal(fmt.Errorf("project not found: %s", input.ProjectID))
	}
	return nil
}

func (s *Store) FindProjectByProjectID(projectID string) (contentmodels.Project, error) {
	if projectID == "" {
		return contentmodels.Project{}, wrapFatal(errors.New("project_id is required"))
	}

	var record contentmodels.Project
	if err := s.db.Where("project_id = ?", projectID).First(&record).Error; err != nil {
		return contentmodels.Project{}, wrapFatal(err)
	}
	return record, nil
}
