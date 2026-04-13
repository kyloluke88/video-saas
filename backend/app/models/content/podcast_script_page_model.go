package content

import (
	"encoding/json"
	"time"

	"api/app/models"
)

type PodcastScriptPage struct {
	models.BaseModel

	Slug             string          `gorm:"column:slug;size:160;uniqueIndex;not null" json:"slug"`
	ProjectID        string          `gorm:"column:project_id;size:120;uniqueIndex;not null" json:"project_id"`
	Language         string          `gorm:"column:language" json:"language"`
	AudienceLanguage string          `gorm:"column:audience_language" json:"audience_language,omitempty"`
	Title            string          `gorm:"column:title" json:"title"`
	EnTitle          string          `gorm:"column:en_title" json:"en_title,omitempty"`
	Subtitle         string          `gorm:"column:subtitle" json:"subtitle,omitempty"`
	Summary          string          `gorm:"column:summary" json:"summary,omitempty"`
	CoverImageURL    string          `gorm:"column:cover_image_url" json:"cover_image_url,omitempty"`
	VideoURL         string          `gorm:"column:video_url" json:"video_url,omitempty"`
	YouTubeVideoID   string          `gorm:"column:youtube_video_id" json:"youtube_video_id,omitempty"`
	YouTubeVideoURL  string          `gorm:"column:youtube_video_url" json:"youtube_video_url,omitempty"`
	SEOTitle         string          `gorm:"column:seo_title" json:"seo_title,omitempty"`
	SEODescription   string          `gorm:"column:seo_description" json:"seo_description,omitempty"`
	SEOKeywords      StringArray     `gorm:"column:seo_keywords;type:text[]" json:"seo_keywords,omitempty"`
	CanonicalURL     string          `gorm:"column:canonical_url" json:"canonical_url,omitempty"`
	Script           json.RawMessage `gorm:"column:script_json;type:jsonb" json:"script"`
	Vocabulary       json.RawMessage `gorm:"column:vocabulary_json;type:jsonb" json:"vocabulary,omitempty"`
	Grammar          json.RawMessage `gorm:"column:grammar_json;type:jsonb" json:"grammar,omitempty"`
	Downloads        json.RawMessage `gorm:"column:downloads_json;type:jsonb" json:"downloads,omitempty"`
	Status           string          `gorm:"column:status" json:"-"`
	PublishedAt      *time.Time      `gorm:"column:published_at" json:"published_at,omitempty"`

	models.CommonTimestampsField
}

func (PodcastScriptPage) TableName() string {
	return "podcast_script_pages"
}
