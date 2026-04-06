package content

import (
	"encoding/json"
	"time"

	"api/app/models"
)

type VideoProject struct {
	models.BaseModel

	ProjectID       string          `gorm:"column:project_id" json:"project_id"`
	ContentType     string          `gorm:"column:content_type" json:"content_type"`
	Language        string          `gorm:"column:language" json:"language,omitempty"`
	Title           string          `gorm:"column:title" json:"title,omitempty"`
	Status          string          `gorm:"column:status" json:"status"`
	CurrentStage    string          `gorm:"column:current_stage" json:"current_stage,omitempty"`
	CurrentTaskType string          `gorm:"column:current_task_type" json:"current_task_type,omitempty"`
	LastError       string          `gorm:"column:last_error" json:"last_error,omitempty"`
	RequestPayload  json.RawMessage `gorm:"column:request_payload_json;type:jsonb" json:"request_payload,omitempty"`
	Metadata        json.RawMessage `gorm:"column:metadata_json;type:jsonb" json:"metadata,omitempty"`
	VideoURL        string          `gorm:"column:video_url" json:"video_url,omitempty"`
	YouTubeVideoID  string          `gorm:"column:youtube_video_id" json:"youtube_video_id,omitempty"`
	YouTubeVideoURL string          `gorm:"column:youtube_video_url" json:"youtube_video_url,omitempty"`
	ScriptPageID    *uint64         `gorm:"column:script_page_id" json:"script_page_id,omitempty"`
	ScriptPageSlug  string          `gorm:"column:script_page_slug" json:"script_page_slug,omitempty"`
	StartedAt       *time.Time      `gorm:"column:started_at" json:"started_at,omitempty"`
	FinishedAt      *time.Time      `gorm:"column:finished_at" json:"finished_at,omitempty"`

	models.CommonTimestampsField
}

func (VideoProject) TableName() string {
	return "video_projects"
}

type VideoProjectTaskRun struct {
	models.BaseModel

	ProjectID    string          `gorm:"column:project_id" json:"project_id"`
	TaskID       string          `gorm:"column:task_id" json:"task_id"`
	TaskType     string          `gorm:"column:task_type" json:"task_type"`
	Stage        string          `gorm:"column:stage" json:"stage"`
	Status       string          `gorm:"column:status" json:"status"`
	RetryCount   int             `gorm:"column:retry_count" json:"retry_count"`
	Payload      json.RawMessage `gorm:"column:payload_json;type:jsonb" json:"payload,omitempty"`
	ErrorMessage string          `gorm:"column:error_message" json:"error_message,omitempty"`
	StartedAt    time.Time       `gorm:"column:started_at" json:"started_at"`
	FinishedAt   *time.Time      `gorm:"column:finished_at" json:"finished_at,omitempty"`

	models.CommonTimestampsField
}

func (VideoProjectTaskRun) TableName() string {
	return "video_project_task_runs"
}
