package podcast_export_service

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	conf "worker/pkg/config"
	podcastpageservice "worker/services/podcast/page"
	podcastspeaker "worker/services/podcast/speaker"

	"github.com/go-pdf/fpdf"
)

const (
	pdfFilename            = "chat_script.pdf"
	youtubePublishFilename = "youtube_publish.txt"
)

type Result struct {
	PDFPath               string
	YouTubePublishPath    string
	YouTubeTranscriptPath string
}

type scriptDocument struct {
	Sections []sectionDocument `json:"sections"`
}

type sectionDocument struct {
	Heading string         `json:"heading,omitempty"`
	Lines   []lineDocument `json:"lines"`
}

type lineDocument struct {
	Speaker     string `json:"speaker"`
	SpeakerName string `json:"speaker_name,omitempty"`
	Text        string `json:"text"`
	Translation string `json:"translation,omitempty"`
}

type vocabularyItem struct {
	Term        string            `json:"term"`
	Meaning     string            `json:"meaning"`
	Explanation string            `json:"explanation"`
	Examples    []exampleDocument `json:"examples,omitempty"`
}

type grammarItem struct {
	Pattern     string            `json:"pattern"`
	Meaning     string            `json:"meaning"`
	Explanation string            `json:"explanation"`
	Examples    []exampleDocument `json:"examples,omitempty"`
}

type exampleDocument struct {
	Text        string `json:"text"`
	Translation string `json:"translation,omitempty"`
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
		PDFPath:               filepath.Join(projectDir, pdfFilename),
		YouTubePublishPath:    filepath.Join(projectDir, youtubePublishFilename),
		YouTubeTranscriptPath: filepath.Join(projectDir, youtubeTranscriptFilename),
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
	if err := writePDF(result.PDFPath, source.Upsert.Language, source.Upsert.Title, source.Upsert.Subtitle, source.Upsert.Summary, doc, vocabulary, grammar); err != nil {
		return Result{}, err
	}
	if err := exportYouTubeAssets(projectDir, source); err != nil {
		return Result{}, err
	}
	return result, nil
}

func writePDF(outputPath, language, title, subtitle, summary string, doc scriptDocument, vocabulary []vocabularyItem, grammar []grammarItem) error {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(14, 16, 14)
	pdf.SetAutoPageBreak(true, 14)
	pdf.AddPage()
	pdf.SetTitle(strings.TrimSpace(title), true)
	pdf.SetAuthor("video-saas", true)

	fontName := "PodcastBody"
	fontPath := fontPathForLanguage(language)
	pdf.SetFontLocation(filepath.Dir(fontPath))
	pdf.AddUTF8Font(fontName, "", filepath.Base(fontPath))
	if err := pdf.Error(); err != nil {
		return err
	}

	writeTitle(pdf, fontName, title, subtitle, summary)
	writeSections(pdf, fontName, doc)
	writeVocabulary(pdf, fontName, vocabulary)
	writeGrammar(pdf, fontName, grammar)

	return pdf.OutputFileAndClose(outputPath)
}

func writeTitle(pdf *fpdf.Fpdf, fontName, title, subtitle, summary string) {
	pdf.SetFont(fontName, "", 22)
	pdf.MultiCell(0, 10, strings.TrimSpace(title), "", "L", false)
	pdf.Ln(2)

	if strings.TrimSpace(subtitle) != "" {
		pdf.SetTextColor(80, 80, 80)
		pdf.SetFont(fontName, "", 12)
		pdf.MultiCell(0, 6, strings.TrimSpace(subtitle), "", "L", false)
		pdf.Ln(1)
		pdf.SetTextColor(0, 0, 0)
	}

	if strings.TrimSpace(summary) != "" {
		pdf.SetFont(fontName, "", 11)
		pdf.MultiCell(0, 6, strings.TrimSpace(summary), "", "L", false)
		pdf.Ln(4)
	}
}

func writeSections(pdf *fpdf.Fpdf, fontName string, doc scriptDocument) {
	for _, section := range doc.Sections {
		if strings.TrimSpace(section.Heading) != "" {
			pdf.SetFont(fontName, "", 16)
			pdf.MultiCell(0, 8, strings.TrimSpace(section.Heading), "", "L", false)
			pdf.Ln(1)
		}
		for _, line := range section.Lines {
			speaker := strings.TrimSpace(line.SpeakerName)
			if speaker == "" {
				speaker = podcastspeaker.PreferredDisplayName(line.Speaker)
			}
			if speaker == "" {
				speaker = strings.TrimSpace(line.Speaker)
			}

			pdf.SetFont(fontName, "", 11)
			pdf.SetTextColor(25, 25, 25)
			pdf.MultiCell(0, 6, fmt.Sprintf("%s: %s", speaker, strings.TrimSpace(line.Text)), "", "L", false)
			if strings.TrimSpace(line.Translation) != "" {
				pdf.SetTextColor(90, 90, 90)
				pdf.SetFont(fontName, "", 9)
				pdf.MultiCell(0, 5, strings.TrimSpace(line.Translation), "", "L", false)
			}
			pdf.SetTextColor(0, 0, 0)
			pdf.Ln(2)
		}
		pdf.Ln(2)
	}
}

func writeVocabulary(pdf *fpdf.Fpdf, fontName string, vocabulary []vocabularyItem) {
	if len(vocabulary) == 0 {
		return
	}
	pdf.AddPage()
	pdf.SetFont(fontName, "", 18)
	pdf.MultiCell(0, 8, "Vocabulary", "", "L", false)
	pdf.Ln(3)
	for _, item := range vocabulary {
		pdf.SetFont(fontName, "", 14)
		pdf.MultiCell(0, 7, strings.TrimSpace(item.Term), "", "L", false)
		pdf.SetFont(fontName, "", 10)
		pdf.MultiCell(0, 6, "Meaning: "+strings.TrimSpace(item.Meaning), "", "L", false)
		pdf.MultiCell(0, 6, "Explanation: "+strings.TrimSpace(item.Explanation), "", "L", false)
		for _, example := range item.Examples {
			if strings.TrimSpace(example.Text) != "" {
				pdf.MultiCell(0, 6, "Example: "+strings.TrimSpace(example.Text), "", "L", false)
			}
			if strings.TrimSpace(example.Translation) != "" {
				pdf.SetTextColor(90, 90, 90)
				pdf.MultiCell(0, 5, strings.TrimSpace(example.Translation), "", "L", false)
				pdf.SetTextColor(0, 0, 0)
			}
		}
		pdf.Ln(3)
	}
}

func writeGrammar(pdf *fpdf.Fpdf, fontName string, grammar []grammarItem) {
	if len(grammar) == 0 {
		return
	}
	pdf.AddPage()
	pdf.SetFont(fontName, "", 18)
	pdf.MultiCell(0, 8, "Grammar", "", "L", false)
	pdf.Ln(3)
	for _, item := range grammar {
		pdf.SetFont(fontName, "", 14)
		pdf.MultiCell(0, 7, strings.TrimSpace(item.Pattern), "", "L", false)
		pdf.SetFont(fontName, "", 10)
		pdf.MultiCell(0, 6, "Meaning: "+strings.TrimSpace(item.Meaning), "", "L", false)
		pdf.MultiCell(0, 6, "Explanation: "+strings.TrimSpace(item.Explanation), "", "L", false)
		for _, example := range item.Examples {
			if strings.TrimSpace(example.Text) != "" {
				pdf.MultiCell(0, 6, "Example: "+strings.TrimSpace(example.Text), "", "L", false)
			}
			if strings.TrimSpace(example.Translation) != "" {
				pdf.SetTextColor(90, 90, 90)
				pdf.MultiCell(0, 5, strings.TrimSpace(example.Translation), "", "L", false)
				pdf.SetTextColor(0, 0, 0)
			}
		}
		pdf.Ln(3)
	}
}

func fontPathForLanguage(language string) string {
	fontRelPath := filepath.Join("fonts", "zh", "hanyiwenrunsongyun.ttf")
	if podcastspeaker.IsJapaneseLanguage(language) {
		fontRelPath = filepath.Join("fonts", "jp", "ZenKurenaido-Regular.ttf")
	}

	candidates := make([]string, 0, 10)
	if configured := strings.TrimSpace(conf.Get[string]("worker.worker_assets_dir")); configured != "" {
		candidates = append(candidates, configured)
	}
	cwd, err := os.Getwd()
	if err == nil {
		for current := cwd; current != "" && current != string(filepath.Separator); current = filepath.Dir(current) {
			candidates = append(candidates, filepath.Join(current, "assets"))
			candidates = append(candidates, filepath.Join(current, "worker", "assets"))
			parent := filepath.Dir(current)
			if parent == current {
				break
			}
		}
	}
	for _, baseDir := range candidates {
		baseDir = strings.TrimSpace(baseDir)
		if baseDir == "" {
			continue
		}
		fullPath := filepath.Join(baseDir, fontRelPath)
		if _, err := os.Stat(fullPath); err == nil {
			return fullPath
		}
	}
	return filepath.Join("assets", fontRelPath)
}
