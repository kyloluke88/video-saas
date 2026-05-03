package podcast_export_service

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	conf "worker/pkg/config"
	podcastpageservice "worker/services/podcast/page"
)

const (
	pdfFilename            = "chat_script.pdf"
	youtubePublishFilename = "youtube_publish.txt"
)

type Result struct {
	PDFPath                string
	YouTubePublishPath     string
	YouTubeTranscriptPaths []string
}

type scriptDocument struct {
	Sections []sectionDocument `json:"sections"`
}

type sectionDocument struct {
	Heading string         `json:"heading,omitempty"`
	Lines   []lineDocument `json:"lines"`
}

type lineDocument struct {
	Speaker     string      `json:"speaker"`
	SpeakerName string      `json:"speaker_name,omitempty"`
	Text        string      `json:"text"`
	Ruby        []rubyToken `json:"ruby,omitempty"`
	Translation string      `json:"translation,omitempty"`
}

type rubyToken struct {
	Surface string `json:"surface"`
	Reading string `json:"reading"`
}

type phoneticToken struct {
	Char    string `json:"char"`
	Reading string `json:"reading"`
}

type vocabularyItem struct {
	Term        string            `json:"term"`
	Tokens      []phoneticToken   `json:"tokens,omitempty"`
	Meaning     string            `json:"meaning"`
	Explanation string            `json:"explanation"`
	Examples    []exampleDocument `json:"examples,omitempty"`
}

type grammarItem struct {
	Pattern     string            `json:"pattern"`
	Tokens      []phoneticToken   `json:"tokens,omitempty"`
	Meaning     string            `json:"meaning"`
	Explanation string            `json:"explanation"`
	Examples    []exampleDocument `json:"examples,omitempty"`
}

type exampleDocument struct {
	Text        string          `json:"text"`
	Tokens      []phoneticToken `json:"tokens,omitempty"`
	Translation string          `json:"translation,omitempty"`
}

func Generate(projectID string) (Result, error) {
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return Result{}, fmt.Errorf("project_id is required")
	}
	projectDir := filepath.Join(conf.Get[string]("worker.ffmpeg_work_dir"), "projects", projectID)
	return GenerateFromProjectDir(projectDir, projectID)
}

func GenerateFromProjectDir(projectDir, projectID string) (Result, error) {
	source, err := podcastpageservice.BuildPageSourceFromProjectDir(projectDir, podcastpageservice.PersistInput{
		ProjectID: strings.TrimSpace(projectID),
	})
	if err != nil {
		return Result{}, err
	}
	return GenerateFromPageSource(source)
}

func GenerateFromPageSource(source podcastpageservice.PageSource) (Result, error) {
	projectDir := strings.TrimSpace(source.ProjectDir)
	if projectDir == "" {
		projectID := strings.TrimSpace(source.Upsert.ProjectID)
		if projectID == "" {
			return Result{}, fmt.Errorf("project_dir or project_id is required")
		}
		projectDir = filepath.Join(conf.Get[string]("worker.ffmpeg_work_dir"), "projects", projectID)
	}

	result := Result{
		PDFPath:            filepath.Join(projectDir, pdfFilename),
		YouTubePublishPath: filepath.Join(projectDir, youtubePublishFilename),
	}

	var doc scriptDocument
	if err := json.Unmarshal(source.Upsert.Script, &doc); err != nil {
		return Result{}, fmt.Errorf("decode script json failed: %w", err)
	}
	var vocabulary []vocabularyItem
	if len(source.Upsert.Vocabulary) > 0 {
		if err := json.Unmarshal(source.Upsert.Vocabulary, &vocabulary); err != nil {
			return Result{}, fmt.Errorf("decode vocabulary json failed: %w", err)
		}
	}
	var grammar []grammarItem
	if len(source.Upsert.Grammar) > 0 {
		if err := json.Unmarshal(source.Upsert.Grammar, &grammar); err != nil {
			return Result{}, fmt.Errorf("decode grammar json failed: %w", err)
		}
	}

	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		return Result{}, err
	}
	if err := renderPodcastPDF(result.PDFPath, source.Upsert.Language, source.Upsert.Title, source.Upsert.Subtitle, source.Upsert.Summary, doc, vocabulary, grammar); err != nil {
		return Result{}, err
	}
	transcriptPaths, err := exportYouTubeAssets(projectDir, source)
	if err != nil {
		return Result{}, err
	}
	result.YouTubeTranscriptPaths = transcriptPaths
	return result, nil
}

func fontPathForLanguage(language string) string {
	switch normalizedLanguage(language) {
	case "ja":
		return resolveFontAssetPath(
			filepath.Join("fonts", "jp", "MarukoGothicCJKjp-Regular.ttf"),
			filepath.Join("fonts", "jp", "ZenKurenaido-Regular.ttf"),
		)
	case "en":
		return resolveFontAssetPath(filepath.Join("fonts", "en", "TenorSans-Regular.ttf"))
	default:
		return resolveFontAssetPath(filepath.Join("fonts", "zh", "hanyiwenrunsongyun.ttf"))
	}
}
