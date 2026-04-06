package persistence

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	conf "worker/pkg/config"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type PodcastScriptPageRecord struct {
	ID               uint64          `gorm:"column:id;primaryKey;autoIncrement"`
	Slug             string          `gorm:"column:slug"`
	ProjectID        string          `gorm:"column:project_id"`
	Language         string          `gorm:"column:language"`
	AudienceLanguage string          `gorm:"column:audience_language"`
	Title            string          `gorm:"column:title"`
	Subtitle         string          `gorm:"column:subtitle"`
	Summary          string          `gorm:"column:summary"`
	CoverImageURL    string          `gorm:"column:cover_image_url"`
	VideoURL         string          `gorm:"column:video_url"`
	YouTubeVideoID   string          `gorm:"column:youtube_video_id"`
	YouTubeVideoURL  string          `gorm:"column:youtube_video_url"`
	SEOTitle         string          `gorm:"column:seo_title"`
	SEODescription   string          `gorm:"column:seo_description"`
	SEOKeywords      []string        `gorm:"column:seo_keywords;type:text[]"`
	CanonicalURL     string          `gorm:"column:canonical_url"`
	Script           json.RawMessage `gorm:"column:script_json;type:jsonb"`
	Vocabulary       json.RawMessage `gorm:"column:vocabulary_json;type:jsonb"`
	Grammar          json.RawMessage `gorm:"column:grammar_json;type:jsonb"`
	Downloads        json.RawMessage `gorm:"column:downloads_json;type:jsonb"`
	Sidebar          json.RawMessage `gorm:"column:sidebar_json;type:jsonb"`
	Status           string          `gorm:"column:status"`
	PublishedAt      *time.Time      `gorm:"column:published_at"`
	CreatedAt        time.Time       `gorm:"column:created_at"`
	UpdatedAt        time.Time       `gorm:"column:updated_at"`
}

func (PodcastScriptPageRecord) TableName() string {
	return "podcast_script_pages"
}

type VideoProjectRecord struct {
	ID              uint64          `gorm:"column:id;primaryKey;autoIncrement"`
	ProjectID       string          `gorm:"column:project_id"`
	ContentType     string          `gorm:"column:content_type"`
	Language        string          `gorm:"column:language"`
	Title           string          `gorm:"column:title"`
	Status          string          `gorm:"column:status"`
	CurrentStage    string          `gorm:"column:current_stage"`
	CurrentTaskType string          `gorm:"column:current_task_type"`
	LastError       string          `gorm:"column:last_error"`
	RequestPayload  json.RawMessage `gorm:"column:request_payload_json;type:jsonb"`
	Metadata        json.RawMessage `gorm:"column:metadata_json;type:jsonb"`
	VideoURL        string          `gorm:"column:video_url"`
	YouTubeVideoID  string          `gorm:"column:youtube_video_id"`
	YouTubeVideoURL string          `gorm:"column:youtube_video_url"`
	ScriptPageID    *uint64         `gorm:"column:script_page_id"`
	ScriptPageSlug  string          `gorm:"column:script_page_slug"`
	StartedAt       *time.Time      `gorm:"column:started_at"`
	FinishedAt      *time.Time      `gorm:"column:finished_at"`
	CreatedAt       time.Time       `gorm:"column:created_at"`
	UpdatedAt       time.Time       `gorm:"column:updated_at"`
}

func (VideoProjectRecord) TableName() string {
	return "video_projects"
}

type VideoProjectTaskRunRecord struct {
	ID           uint64          `gorm:"column:id;primaryKey;autoIncrement"`
	ProjectID    string          `gorm:"column:project_id"`
	TaskID       string          `gorm:"column:task_id"`
	TaskType     string          `gorm:"column:task_type"`
	Stage        string          `gorm:"column:stage"`
	Status       string          `gorm:"column:status"`
	RetryCount   int             `gorm:"column:retry_count"`
	Payload      json.RawMessage `gorm:"column:payload_json;type:jsonb"`
	ErrorMessage string          `gorm:"column:error_message"`
	StartedAt    time.Time       `gorm:"column:started_at"`
	FinishedAt   *time.Time      `gorm:"column:finished_at"`
	CreatedAt    time.Time       `gorm:"column:created_at"`
	UpdatedAt    time.Time       `gorm:"column:updated_at"`
}

func (VideoProjectTaskRunRecord) TableName() string {
	return "video_project_task_runs"
}

type Store struct {
	db *gorm.DB
}

type ScriptPageUpsert struct {
	Slug             string
	ProjectID        string
	Language         string
	AudienceLanguage string
	Title            string
	Subtitle         string
	Summary          string
	CoverImageURL    string
	VideoURL         string
	YouTubeVideoID   string
	YouTubeVideoURL  string
	SEOTitle         string
	SEODescription   string
	SEOKeywords      []string
	CanonicalURL     string
	Script           json.RawMessage
	Vocabulary       json.RawMessage
	Grammar          json.RawMessage
	Downloads        json.RawMessage
	Sidebar          json.RawMessage
	Status           string
	PublishedAt      *time.Time
}

type VideoProjectUpsert struct {
	ProjectID       string
	ContentType     string
	Language        string
	Title           string
	Status          string
	CurrentStage    string
	CurrentTaskType string
	LastError       string
	RequestPayload  json.RawMessage
	Metadata        json.RawMessage
	VideoURL        string
	YouTubeVideoID  string
	YouTubeVideoURL string
	ScriptPageID    *uint64
	ScriptPageSlug  string
	StartedAt       *time.Time
	FinishedAt      *time.Time
}

type TaskRunUpsert struct {
	ProjectID    string
	TaskID       string
	TaskType     string
	Stage        string
	Status       string
	RetryCount   int
	Payload      json.RawMessage
	ErrorMessage string
	StartedAt    time.Time
	FinishedAt   *time.Time
}

var (
	defaultStoreMu sync.Mutex
	defaultStore   *Store
)

func DefaultStore() (*Store, error) {
	defaultStoreMu.Lock()
	defer defaultStoreMu.Unlock()

	if defaultStore != nil {
		return defaultStore, nil
	}

	db, err := openDB()
	if err != nil {
		return nil, err
	}
	defaultStore = &Store{db: db}
	return defaultStore, nil
}

func openDB() (*gorm.DB, error) {
	if conf.Get[string]("worker.db_connection") != "postgresql" {
		return nil, fmt.Errorf("unsupported worker.db_connection=%s", conf.Get[string]("worker.db_connection"))
	}

	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s dbname=%s password=%s sslmode=disable",
		conf.Get[string]("worker.db_host"),
		conf.Get[string]("worker.db_port"),
		conf.Get[string]("worker.db_username"),
		conf.Get[string]("worker.db_database"),
		conf.Get[string]("worker.db_password"),
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxIdleConns(conf.Get[int]("worker.db_max_idle_connections"))
	sqlDB.SetMaxOpenConns(conf.Get[int]("worker.db_max_open_connections"))
	sqlDB.SetConnMaxLifetime(time.Duration(conf.Get[int]("worker.db_max_life_seconds")) * time.Second)

	return db, nil
}

func (s *Store) UpsertPodcastScriptPage(input ScriptPageUpsert) (uint64, error) {
	if input.ProjectID == "" {
		return 0, errors.New("project_id is required")
	}

	record := PodcastScriptPageRecord{
		Slug:             input.Slug,
		ProjectID:        input.ProjectID,
		Language:         input.Language,
		AudienceLanguage: input.AudienceLanguage,
		Title:            input.Title,
		Subtitle:         input.Subtitle,
		Summary:          input.Summary,
		CoverImageURL:    input.CoverImageURL,
		VideoURL:         input.VideoURL,
		YouTubeVideoID:   input.YouTubeVideoID,
		YouTubeVideoURL:  input.YouTubeVideoURL,
		SEOTitle:         input.SEOTitle,
		SEODescription:   input.SEODescription,
		SEOKeywords:      input.SEOKeywords,
		CanonicalURL:     input.CanonicalURL,
		Script:           defaultJSON(input.Script, []byte(`{"intro":"","sections":[]}`)),
		Vocabulary:       defaultJSON(input.Vocabulary, []byte(`[]`)),
		Grammar:          defaultJSON(input.Grammar, []byte(`[]`)),
		Downloads:        defaultJSON(input.Downloads, []byte(`[]`)),
		Sidebar:          defaultJSON(input.Sidebar, []byte(`{"products":[]}`)),
		Status:           defaultString(input.Status, "published"),
		PublishedAt:      input.PublishedAt,
	}

	var existing PodcastScriptPageRecord
	err := s.db.Where("project_id = ?", input.ProjectID).First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		if err := s.db.Create(&record).Error; err != nil {
			return 0, err
		}
		return record.ID, nil
	}
	if err != nil {
		return 0, err
	}

	record.ID = existing.ID
	if err := s.db.Model(&PodcastScriptPageRecord{}).
		Where("id = ?", existing.ID).
		Updates(map[string]interface{}{
			"slug":              record.Slug,
			"language":          record.Language,
			"audience_language": record.AudienceLanguage,
			"title":             record.Title,
			"subtitle":          record.Subtitle,
			"summary":           record.Summary,
			"cover_image_url":   record.CoverImageURL,
			"video_url":         record.VideoURL,
			"youtube_video_id":  record.YouTubeVideoID,
			"youtube_video_url": record.YouTubeVideoURL,
			"seo_title":         record.SEOTitle,
			"seo_description":   record.SEODescription,
			"seo_keywords":      record.SEOKeywords,
			"canonical_url":     record.CanonicalURL,
			"script_json":       record.Script,
			"vocabulary_json":   record.Vocabulary,
			"grammar_json":      record.Grammar,
			"downloads_json":    record.Downloads,
			"sidebar_json":      record.Sidebar,
			"status":            record.Status,
			"published_at":      record.PublishedAt,
		}).
		Error; err != nil {
		return 0, err
	}
	return existing.ID, nil
}

func (s *Store) UpsertVideoProject(input VideoProjectUpsert) error {
	if input.ProjectID == "" {
		return errors.New("project_id is required")
	}

	var existing VideoProjectRecord
	err := s.db.Where("project_id = ?", input.ProjectID).First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		record := VideoProjectRecord{
			ProjectID:       input.ProjectID,
			ContentType:     defaultString(input.ContentType, "podcast"),
			Language:        input.Language,
			Title:           input.Title,
			Status:          defaultString(input.Status, "queued"),
			CurrentStage:    input.CurrentStage,
			CurrentTaskType: input.CurrentTaskType,
			LastError:       input.LastError,
			RequestPayload:  defaultJSON(input.RequestPayload, []byte(`{}`)),
			Metadata:        defaultJSON(input.Metadata, []byte(`{}`)),
			VideoURL:        input.VideoURL,
			YouTubeVideoID:  input.YouTubeVideoID,
			YouTubeVideoURL: input.YouTubeVideoURL,
			ScriptPageID:    input.ScriptPageID,
			ScriptPageSlug:  input.ScriptPageSlug,
			StartedAt:       input.StartedAt,
			FinishedAt:      input.FinishedAt,
		}
		return s.db.Create(&record).Error
	}
	if err != nil {
		return err
	}

	updates := map[string]interface{}{
		"content_type":      defaultString(input.ContentType, existing.ContentType),
		"language":          coalesceString(input.Language, existing.Language),
		"title":             coalesceString(input.Title, existing.Title),
		"status":            defaultString(input.Status, existing.Status),
		"current_stage":     input.CurrentStage,
		"current_task_type": input.CurrentTaskType,
		"last_error":        input.LastError,
		"video_url":         coalesceString(input.VideoURL, existing.VideoURL),
		"youtube_video_id":  coalesceString(input.YouTubeVideoID, existing.YouTubeVideoID),
		"youtube_video_url": coalesceString(input.YouTubeVideoURL, existing.YouTubeVideoURL),
		"script_page_id":    coalesceUint64Ptr(input.ScriptPageID, existing.ScriptPageID),
		"script_page_slug":  coalesceString(input.ScriptPageSlug, existing.ScriptPageSlug),
	}
	if len(input.RequestPayload) > 0 {
		updates["request_payload_json"] = input.RequestPayload
	}
	if len(input.Metadata) > 0 {
		updates["metadata_json"] = input.Metadata
	}
	if input.StartedAt != nil {
		updates["started_at"] = input.StartedAt
	}
	if input.FinishedAt != nil {
		updates["finished_at"] = input.FinishedAt
	}

	return s.db.Model(&VideoProjectRecord{}).
		Where("project_id = ?", input.ProjectID).
		Updates(updates).
		Error
}

func (s *Store) UpsertTaskRun(input TaskRunUpsert) error {
	if input.ProjectID == "" || input.TaskID == "" {
		return errors.New("project_id and task_id are required")
	}

	record := VideoProjectTaskRunRecord{
		ProjectID:    input.ProjectID,
		TaskID:       input.TaskID,
		TaskType:     input.TaskType,
		Stage:        input.Stage,
		Status:       input.Status,
		RetryCount:   input.RetryCount,
		Payload:      defaultJSON(input.Payload, []byte(`{}`)),
		ErrorMessage: input.ErrorMessage,
		StartedAt:    input.StartedAt,
		FinishedAt:   input.FinishedAt,
	}

	return s.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "task_id"}, {Name: "retry_count"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"project_id":    record.ProjectID,
			"task_type":     record.TaskType,
			"stage":         record.Stage,
			"status":        record.Status,
			"payload_json":  record.Payload,
			"error_message": record.ErrorMessage,
			"finished_at":   record.FinishedAt,
			"updated_at":    time.Now().UTC(),
		}),
	}).Create(&record).Error
}

func defaultJSON(value json.RawMessage, fallback []byte) json.RawMessage {
	if len(value) == 0 {
		return json.RawMessage(fallback)
	}
	return value
}

func defaultString(value string, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func coalesceString(value string, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func coalesceUint64Ptr(value *uint64, fallback *uint64) *uint64 {
	if value != nil {
		return value
	}
	return fallback
}
