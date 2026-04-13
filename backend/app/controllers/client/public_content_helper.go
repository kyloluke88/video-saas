package client

import (
	"strconv"
	"strings"
	"time"

	contentModel "api/app/models/content"
	"api/pkg/database"
	"api/pkg/logger"
)

type PublicPodcastScriptListItem struct {
	ID              uint64     `gorm:"column:id" json:"id"`
	Slug            string     `gorm:"column:slug" json:"slug"`
	Title           string     `gorm:"column:title" json:"title"`
	EnTitle         string     `gorm:"column:en_title" json:"en_title,omitempty"`
	Summary         string     `gorm:"column:summary" json:"summary,omitempty"`
	YouTubeVideoID  string     `gorm:"column:youtube_video_id" json:"youtube_video_id,omitempty"`
	YouTubeVideoURL string     `gorm:"column:youtube_video_url" json:"youtube_video_url,omitempty"`
	PublishedAt     *time.Time `gorm:"column:published_at" json:"published_at,omitempty"`
}

func parsePositiveIntQuery(raw string, fallback, minValue, maxValue int) int {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || value < minValue {
		return fallback
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func normalizePublicLocale(raw string) (locale string, ok bool) {
	locale = strings.ToLower(strings.TrimSpace(raw))
	if locale == "" {
		return "", true
	}
	switch locale {
	case "zh", "ja":
		return locale, true
	default:
		return "", false
	}
}

func listPublishedPodcastScripts(language string, limit int) ([]PublicPodcastScriptListItem, error) {
	query := database.DB.
		Model(&contentModel.PodcastScriptPage{}).
		Select("id", "slug", "title", "en_title", "summary", "youtube_video_id", "youtube_video_url", "published_at").
		Where("status = ?", "published")

	if language != "" {
		query = query.Where("language = ?", language)
	}

	var pages []PublicPodcastScriptListItem

	err := query.
		Order("published_at DESC NULLS LAST").
		Order("created_at DESC").
		Limit(limit).
		Find(&pages).
		Error

	logger.Dump(pages)
	return pages, err
}
