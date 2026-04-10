package content

import (
	"encoding/json"
	"time"

	"api/app/models"
	"api/pkg/database"

	"gorm.io/gorm/clause"
)

const (
	// ProjectStatusQueued = 0：项目已创建，任务已入队或准备入队，尚未开始实际执行。
	ProjectStatusQueued int16 = iota
	// ProjectStatusRunning = 1：当前项目正在执行某个任务阶段。
	ProjectStatusRunning
	// ProjectStatusRetrying = 2：当前项目某个任务失败后，已经进入重试等待或重试执行状态。
	ProjectStatusRetrying
	// ProjectStatusFinished = 3：当前项目整条执行链已成功结束。
	ProjectStatusFinished
	// ProjectStatusError = 4：当前项目在某个任务阶段失败并终止。
	ProjectStatusError
	// ProjectStatusCancelling = 5：当前项目已收到取消请求，正在停止执行中的任务。
	ProjectStatusCancelling
	// ProjectStatusCancelled = 6：当前项目被人工取消或系统取消，且已完成终止。
	ProjectStatusCancelled
)

const (
	ProjectCancelSourceManualAPI = "manual_api"
)

type Project struct {
	models.BaseModel

	ProjectID          string          `gorm:"column:project_id" json:"project_id"`
	ContentType        string          `gorm:"column:content_type" json:"content_type"`
	RunMode            *int            `gorm:"column:run_mode" json:"run_mode,omitempty"`
	RetryNum           int             `gorm:"column:retry_num" json:"retry_num"`
	CurrentStage       string          `gorm:"column:current_stage" json:"current_stage,omitempty"`
	CurrentTaskType    string          `gorm:"column:current_task_type" json:"current_task_type,omitempty"`
	TerminatedTaskType string          `gorm:"column:terminated_task_type" json:"terminated_task_type,omitempty"`
	Status             int16           `gorm:"column:status" json:"status"`
	LastError          string          `gorm:"column:last_error" json:"last_error,omitempty"`
	Payload            json.RawMessage `gorm:"column:payload_json;type:jsonb" json:"payload,omitempty"`
	StartedAt          *time.Time      `gorm:"column:started_at" json:"started_at,omitempty"`
	FinishedAt         *time.Time      `gorm:"column:finished_at" json:"finished_at,omitempty"`
	CancelRequestedAt  *time.Time      `gorm:"column:cancel_requested_at" json:"cancel_requested_at,omitempty"`
	CancelledAt        *time.Time      `gorm:"column:cancelled_at" json:"cancelled_at,omitempty"`
	CancelSource       string          `gorm:"column:cancel_source" json:"cancel_source,omitempty"`

	models.CommonTimestampsField
}

func (Project) TableName() string {
	return "projects"
}

func UpsertProject(project Project, now time.Time) error {
	assignments := map[string]interface{}{
		"content_type":         project.ContentType,
		"retry_num":            project.RetryNum,
		"status":               project.Status,
		"current_stage":        project.CurrentStage,
		"current_task_type":    project.CurrentTaskType,
		"terminated_task_type": project.TerminatedTaskType,
		"last_error":           project.LastError,
		"updated_at":           now,
	}
	if project.RunMode != nil {
		assignments["run_mode"] = *project.RunMode
	}
	if len(project.Payload) > 0 {
		assignments["payload_json"] = project.Payload
	}
	if project.StartedAt != nil {
		assignments["started_at"] = clause.Expr{SQL: "COALESCE(projects.started_at, EXCLUDED.started_at)"}
	}
	if project.FinishedAt != nil {
		assignments["finished_at"] = project.FinishedAt
	}
	if project.CancelRequestedAt != nil {
		assignments["cancel_requested_at"] = project.CancelRequestedAt
	}
	if project.CancelledAt != nil {
		assignments["cancelled_at"] = project.CancelledAt
	}
	if project.CancelSource != "" {
		assignments["cancel_source"] = project.CancelSource
	}

	return database.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "project_id"}},
		DoUpdates: clause.Assignments(assignments),
	}).Create(&project).Error
}

func FindProjectByProjectID(projectID string) (Project, error) {
	var project Project
	err := database.DB.Where("project_id = ?", projectID).First(&project).Error
	return project, err
}

func UpdateProjectByProjectID(projectID string, updates map[string]interface{}) error {
	return database.DB.Model(&Project{}).
		Where("project_id = ?", projectID).
		Updates(updates).
		Error
}

func ProjectStatusName(status int16) string {
	switch status {
	case ProjectStatusQueued:
		return "queued"
	case ProjectStatusRunning:
		return "running"
	case ProjectStatusRetrying:
		return "retrying"
	case ProjectStatusFinished:
		return "finished"
	case ProjectStatusError:
		return "error"
	case ProjectStatusCancelling:
		return "cancelling"
	case ProjectStatusCancelled:
		return "cancelled"
	default:
		return "unknown"
	}
}

func IsTerminalProjectStatus(status int16) bool {
	switch status {
	case ProjectStatusFinished, ProjectStatusError, ProjectStatusCancelled:
		return true
	default:
		return false
	}
}
