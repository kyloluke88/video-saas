package content

import (
	"encoding/json"
	"time"

	"worker/internal/persistence/models"
)

type Project struct {
	models.BaseModel

	ProjectID          string          `gorm:"column:project_id"`
	ContentType        string          `gorm:"column:content_type"`
	RunMode            *int            `gorm:"column:run_mode"`
	RetryNum           int             `gorm:"column:retry_num"`
	CurrentStage       string          `gorm:"column:current_stage"`
	CurrentTaskType    string          `gorm:"column:current_task_type"`
	TerminatedTaskType string          `gorm:"column:terminated_task_type"`
	Status             int16           `gorm:"column:status"`
	LastError          string          `gorm:"column:last_error"`
	Payload            json.RawMessage `gorm:"column:payload_json;type:jsonb"`
	StartedAt          *time.Time      `gorm:"column:started_at"`
	FinishedAt         *time.Time      `gorm:"column:finished_at"`
	CancelRequestedAt  *time.Time      `gorm:"column:cancel_requested_at"`
	CancelledAt        *time.Time      `gorm:"column:cancelled_at"`
	CancelSource       string          `gorm:"column:cancel_source"`

	models.CommonTimestampsField
}

func (Project) TableName() string {
	return "projects"
}
