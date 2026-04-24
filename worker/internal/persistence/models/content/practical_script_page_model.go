package content

import (
	"encoding/json"
	"time"

	"worker/internal/persistence/models"
)

type PracticalScriptPage struct {
	models.BaseModel

	Slug               string          `gorm:"column:slug;size:160;uniqueIndex;not null"`
	ProjectID          string          `gorm:"column:project_id;size:120;uniqueIndex;not null"`
	Language           string          `gorm:"column:language;size:16;not null"`
	AudienceLanguage   string          `gorm:"column:audience_language;size:16"`
	Title              string          `gorm:"column:title;not null"`
	EnTitle            string          `gorm:"column:en_title"`
	Subtitle           string          `gorm:"column:subtitle"`
	Summary            string          `gorm:"column:summary"`
	CoverImageURL      string          `gorm:"column:cover_image_url"`
	VideoURL           string          `gorm:"column:video_url"`
	YouTubeVideoID     string          `gorm:"column:youtube_video_id;size:32"`
	YouTubeVideoURL    string          `gorm:"column:youtube_video_url"`
	TranslationLocales StringArray     `gorm:"column:translation_locales;type:text[]"`
	SEOTitle           string          `gorm:"column:seo_title"`
	SEODescription     string          `gorm:"column:seo_description"`
	SEOKeywords        StringArray     `gorm:"column:seo_keywords;type:text[]"`
	CanonicalURL       string          `gorm:"column:canonical_url"`
	Script             json.RawMessage `gorm:"column:script_json;type:jsonb"`
	Vocabulary         json.RawMessage `gorm:"column:vocabulary_json;type:jsonb"`
	Grammar            json.RawMessage `gorm:"column:grammar_json;type:jsonb"`
	Downloads          json.RawMessage `gorm:"column:downloads_json;type:jsonb"`
	Status             string          `gorm:"column:status"`
	PublishedAt        *time.Time      `gorm:"column:published_at"`

	models.CommonTimestampsField
}

func (PracticalScriptPage) TableName() string {
	return "practical_script_pages"
}
