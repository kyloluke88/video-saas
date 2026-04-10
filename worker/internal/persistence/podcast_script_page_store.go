package persistence

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	contentmodels "worker/internal/persistence/models/content"
)

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
	Status           string
	PublishedAt      *time.Time
}

func (s *Store) UpsertPodcastScriptPage(input ScriptPageUpsert) (uint64, error) {
	if input.ProjectID == "" {
		return 0, wrapFatal(errors.New("project_id is required"))
	}

	record := contentmodels.PodcastScriptPage{
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
		SEOKeywords:      contentmodels.StringArray(input.SEOKeywords),
		CanonicalURL:     input.CanonicalURL,
		Script:           defaultJSON(input.Script, []byte(`{"sections":[]}`)),
		Vocabulary:       defaultJSON(input.Vocabulary, []byte(`[]`)),
		Grammar:          defaultJSON(input.Grammar, []byte(`[]`)),
		Downloads:        defaultJSON(input.Downloads, []byte(`[]`)),
		Status:           defaultString(input.Status, "published"),
		PublishedAt:      input.PublishedAt,
	}

	existing, found, err := s.findPodcastScriptPageForUpsert(input.ProjectID, input.Slug)
	if err != nil {
		return 0, err
	}
	if !found {
		if err := s.db.Create(&record).Error; err != nil {
			return 0, wrapFatal(err)
		}
		return record.ID, nil
	}

	record.ID = existing.ID
	updates := map[string]interface{}{
		"slug":              coalesceString(record.Slug, existing.Slug),
		"project_id":        record.ProjectID,
		"language":          coalesceString(record.Language, existing.Language),
		"audience_language": coalesceString(record.AudienceLanguage, existing.AudienceLanguage),
		"title":             coalesceString(record.Title, existing.Title),
		"subtitle":          coalesceString(record.Subtitle, existing.Subtitle),
		"summary":           coalesceString(record.Summary, existing.Summary),
		"cover_image_url":   coalesceString(record.CoverImageURL, existing.CoverImageURL),
		"video_url":         coalesceString(record.VideoURL, existing.VideoURL),
		"youtube_video_id":  coalesceString(record.YouTubeVideoID, existing.YouTubeVideoID),
		"youtube_video_url": coalesceString(record.YouTubeVideoURL, existing.YouTubeVideoURL),
		"seo_title":         coalesceString(record.SEOTitle, existing.SEOTitle),
		"seo_description":   coalesceString(record.SEODescription, existing.SEODescription),
		"canonical_url":     coalesceString(record.CanonicalURL, existing.CanonicalURL),
		"status":            defaultString(record.Status, existing.Status),
		"published_at":      coalesceTimePtr(record.PublishedAt, existing.PublishedAt),
	}
	if len(record.SEOKeywords) > 0 {
		updates["seo_keywords"] = record.SEOKeywords
	}
	if len(record.Script) > 0 {
		updates["script_json"] = record.Script
	}
	if len(record.Vocabulary) > 0 {
		updates["vocabulary_json"] = record.Vocabulary
	}
	if len(record.Grammar) > 0 {
		updates["grammar_json"] = record.Grammar
	}
	if len(record.Downloads) > 0 {
		updates["downloads_json"] = record.Downloads
	}

	result := s.db.Model(&contentmodels.PodcastScriptPage{}).
		Where("id = ?", existing.ID).
		Updates(updates)
	if result.Error != nil {
		return 0, wrapFatal(result.Error)
	}
	if result.RowsAffected == 0 {
		return 0, wrapFatal(fmt.Errorf("podcast script page update affected 0 rows for project_id: %s slug: %s", input.ProjectID, input.Slug))
	}
	return existing.ID, nil
}

func (s *Store) findPodcastScriptPageForUpsert(projectID string, slug string) (contentmodels.PodcastScriptPage, bool, error) {
	var bySlug contentmodels.PodcastScriptPage
	slugFound := false
	if slug != "" {
		result := s.db.Where("slug = ?", slug).Limit(1).Find(&bySlug)
		if result.Error != nil {
			return contentmodels.PodcastScriptPage{}, false, wrapFatal(result.Error)
		}
		slugFound = result.RowsAffected > 0
	}

	var byProject contentmodels.PodcastScriptPage
	projectFound := false
	if projectID != "" {
		result := s.db.Where("project_id = ?", projectID).Limit(1).Find(&byProject)
		if result.Error != nil {
			return contentmodels.PodcastScriptPage{}, false, wrapFatal(result.Error)
		}
		projectFound = result.RowsAffected > 0
	}

	switch {
	case slugFound && projectFound && bySlug.ID != byProject.ID:
		return contentmodels.PodcastScriptPage{}, false, wrapFatal(fmt.Errorf("podcast script page conflict: slug %q belongs to project %q while project_id %q points to another page", slug, bySlug.ProjectID, projectID))
	case slugFound:
		return bySlug, true, nil
	case projectFound:
		return byProject, true, nil
	default:
		return contentmodels.PodcastScriptPage{}, false, nil
	}
}

func (s *Store) UpdatePodcastScriptPageDownloads(projectID string, downloads json.RawMessage) error {
	if strings.TrimSpace(projectID) == "" {
		return wrapFatal(errors.New("project_id is required"))
	}
	result := s.db.Model(&contentmodels.PodcastScriptPage{}).
		Where("project_id = ?", strings.TrimSpace(projectID)).
		Updates(map[string]interface{}{
			"downloads_json": defaultJSON(downloads, []byte(`[]`)),
		})
	if result.Error != nil {
		return wrapFatal(result.Error)
	}
	if result.RowsAffected == 0 {
		return wrapFatal(fmt.Errorf("podcast script page not found for project_id: %s", projectID))
	}
	return nil
}
