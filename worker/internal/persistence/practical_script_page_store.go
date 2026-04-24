package persistence

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	contentmodels "worker/internal/persistence/models/content"
)

type PracticalScriptPageUpsert struct {
	Slug               string
	ProjectID          string
	Language           string
	AudienceLanguage   string
	Title              string
	EnTitle            string
	Subtitle           string
	Summary            string
	CoverImageURL      string
	VideoURL           string
	YouTubeVideoID     string
	YouTubeVideoURL    string
	TranslationLocales []string
	SEOTitle           string
	SEODescription     string
	SEOKeywords        []string
	CanonicalURL       string
	Script             json.RawMessage
	Vocabulary         json.RawMessage
	Grammar            json.RawMessage
	Downloads          json.RawMessage
	Status             string
	PublishedAt        *time.Time
}

func buildPracticalScriptPageRecord(input PracticalScriptPageUpsert) contentmodels.PracticalScriptPage {
	return contentmodels.PracticalScriptPage{
		Slug:               input.Slug,
		ProjectID:          input.ProjectID,
		Language:           input.Language,
		AudienceLanguage:   input.AudienceLanguage,
		Title:              input.Title,
		EnTitle:            input.EnTitle,
		Subtitle:           input.Subtitle,
		Summary:            input.Summary,
		CoverImageURL:      input.CoverImageURL,
		VideoURL:           input.VideoURL,
		YouTubeVideoID:     input.YouTubeVideoID,
		YouTubeVideoURL:    input.YouTubeVideoURL,
		TranslationLocales: contentmodels.StringArray(input.TranslationLocales),
		SEOTitle:           input.SEOTitle,
		SEODescription:     input.SEODescription,
		SEOKeywords:        contentmodels.StringArray(input.SEOKeywords),
		CanonicalURL:       input.CanonicalURL,
		Script:             defaultJSON(input.Script, []byte(`{}`)),
		Vocabulary:         defaultJSON(input.Vocabulary, []byte(`[]`)),
		Grammar:            defaultJSON(input.Grammar, []byte(`[]`)),
		Downloads:          defaultJSON(input.Downloads, []byte(`[]`)),
		Status:             defaultString(input.Status, "published"),
		PublishedAt:        input.PublishedAt,
	}
}

func (s *Store) UpsertPracticalScriptPage(input PracticalScriptPageUpsert) (uint64, error) {
	if strings.TrimSpace(input.ProjectID) == "" {
		return 0, wrapFatal(errors.New("project_id is required"))
	}
	if strings.TrimSpace(input.Slug) == "" {
		return 0, wrapFatal(errors.New("slug is required"))
	}

	record := buildPracticalScriptPageRecord(input)
	existing, found, err := s.findPracticalScriptPageForUpsert(input.ProjectID, input.Slug)
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
	updates := buildPracticalScriptPageUpdates(existing, record)
	result := s.db.Model(&contentmodels.PracticalScriptPage{}).
		Where("id = ?", existing.ID).
		Updates(updates)
	if result.Error != nil {
		return 0, wrapFatal(result.Error)
	}
	if result.RowsAffected == 0 {
		return 0, wrapFatal(fmt.Errorf("practical script page update affected 0 rows for project_id: %s slug: %s", input.ProjectID, input.Slug))
	}
	return existing.ID, nil
}

func (s *Store) findPracticalScriptPageForUpsert(projectID, slug string) (contentmodels.PracticalScriptPage, bool, error) {
	var bySlug contentmodels.PracticalScriptPage
	slugFound := false
	if slug != "" {
		result := s.db.Where("slug = ?", strings.TrimSpace(slug)).Limit(1).Find(&bySlug)
		if result.Error != nil {
			return contentmodels.PracticalScriptPage{}, false, wrapFatal(result.Error)
		}
		slugFound = result.RowsAffected > 0
	}

	var byProject contentmodels.PracticalScriptPage
	projectFound := false
	if projectID != "" {
		result := s.db.Where("project_id = ?", strings.TrimSpace(projectID)).Limit(1).Find(&byProject)
		if result.Error != nil {
			return contentmodels.PracticalScriptPage{}, false, wrapFatal(result.Error)
		}
		projectFound = result.RowsAffected > 0
	}

	switch {
	case slugFound && projectFound && bySlug.ID != byProject.ID:
		return contentmodels.PracticalScriptPage{}, false, wrapFatal(fmt.Errorf("practical script page conflict: slug %q belongs to project %q while project_id %q points to another page", slug, bySlug.ProjectID, projectID))
	case slugFound:
		return bySlug, true, nil
	case projectFound:
		return byProject, true, nil
	default:
		return contentmodels.PracticalScriptPage{}, false, nil
	}
}

func buildPracticalScriptPageUpdates(existing contentmodels.PracticalScriptPage, record contentmodels.PracticalScriptPage) map[string]interface{} {
	updates := map[string]interface{}{
		"slug":                coalesceString(record.Slug, existing.Slug),
		"project_id":          record.ProjectID,
		"language":            coalesceString(record.Language, existing.Language),
		"audience_language":   coalesceString(record.AudienceLanguage, existing.AudienceLanguage),
		"title":               coalesceString(record.Title, existing.Title),
		"en_title":            coalesceString(record.EnTitle, existing.EnTitle),
		"subtitle":            coalesceString(record.Subtitle, existing.Subtitle),
		"summary":             coalesceString(record.Summary, existing.Summary),
		"cover_image_url":     coalesceString(record.CoverImageURL, existing.CoverImageURL),
		"video_url":           coalesceString(record.VideoURL, existing.VideoURL),
		"youtube_video_id":    coalesceString(record.YouTubeVideoID, existing.YouTubeVideoID),
		"youtube_video_url":   coalesceString(record.YouTubeVideoURL, existing.YouTubeVideoURL),
		"seo_title":           coalesceString(record.SEOTitle, existing.SEOTitle),
		"seo_description":     coalesceString(record.SEODescription, existing.SEODescription),
		"canonical_url":       coalesceString(record.CanonicalURL, existing.CanonicalURL),
		"status":              defaultString(record.Status, existing.Status),
		"published_at":        coalesceTimePtr(record.PublishedAt, existing.PublishedAt),
		"translation_locales": record.TranslationLocales,
		"seo_keywords":        record.SEOKeywords,
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
	return updates
}
