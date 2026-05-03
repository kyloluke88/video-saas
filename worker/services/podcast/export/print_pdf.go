package podcast_export_service

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	conf "worker/pkg/config"
	podcastspeaker "worker/services/podcast/speaker"
)

//go:embed templates/podcast_print.html
var printTemplateFS embed.FS

var podcastPrintTemplate = template.Must(template.New("podcast_print.html").ParseFS(printTemplateFS, "templates/podcast_print.html"))

type printTheme struct {
	Accent       string
	AccentSoft   string
	AccentStrong string
	Paper        string
	Panel        string
	Border       string
	Ink          string
	Muted        string
}

type printLabels struct {
	Script     string
	Vocabulary string
	Grammar    string
}

type printView struct {
	Language       string
	Title          string
	Subtitle       string
	Description    []string
	ScriptHeading  string
	VocabularyText string
	GrammarText    string
	Sections       []printSection
	Vocabulary     []printCard
	Grammar        []printCard
	BodyFontURL    string
	DisplayFontURL string
	Theme          printTheme
}

type printSection struct {
	Heading string
	Lines   []printLine
}

type printLine struct {
	Speaker      string
	SpeakerClass string
	TextHTML     template.HTML
	Translation  string
}

type printCard struct {
	HeadlineHTML template.HTML
	Meaning      string
	Explanation  string
	Examples     []printExample
}

type printExample struct {
	TextHTML    template.HTML
	Translation string
}

func renderPodcastPDF(outputPath, language, title, subtitle, summary string, doc scriptDocument, vocabulary []vocabularyItem, grammar []grammarItem) error {
	absOutputPath, err := filepath.Abs(outputPath)
	if err != nil {
		return fmt.Errorf("resolve pdf output path failed: %w", err)
	}
	htmlPath := filepath.Join(filepath.Dir(absOutputPath), strings.TrimSuffix(filepath.Base(absOutputPath), filepath.Ext(absOutputPath))+".print.html")

	view, err := buildPrintView(language, title, subtitle, summary, doc, vocabulary, grammar)
	if err != nil {
		return err
	}
	if err := writePrintHTML(htmlPath, view); err != nil {
		return err
	}
	if err := printHTMLToPDF(htmlPath, absOutputPath); err != nil {
		return err
	}
	return nil
}

func buildPrintView(language, title, subtitle, summary string, doc scriptDocument, vocabulary []vocabularyItem, grammar []grammarItem) (printView, error) {
	bodyFontURL, err := fileURL(fontPathForLanguage(language))
	if err != nil {
		return printView{}, err
	}
	displayFontURL, err := fileURL(displayFontPathForLanguage(language))
	if err != nil {
		return printView{}, err
	}

	labels := localizedPrintLabels(language)
	view := printView{
		Language:       strings.ToUpper(strings.TrimSpace(language)),
		Title:          strings.TrimSpace(title),
		Subtitle:       strings.TrimSpace(subtitle),
		Description:    summaryParagraphs(summary),
		ScriptHeading:  labels.Script,
		VocabularyText: labels.Vocabulary,
		GrammarText:    labels.Grammar,
		BodyFontURL:    bodyFontURL,
		DisplayFontURL: displayFontURL,
		Theme:          printThemeForLanguage(language),
	}
	if len(view.Description) == 0 && view.Subtitle != "" {
		view.Description = []string{view.Subtitle}
		view.Subtitle = ""
	}

	view.Sections = make([]printSection, 0, len(doc.Sections))
	for _, section := range doc.Sections {
		lines := make([]printLine, 0, len(section.Lines))
		for _, line := range section.Lines {
			content := strings.TrimSpace(line.Text)
			if content == "" {
				continue
			}
			speaker := strings.TrimSpace(line.SpeakerName)
			if speaker == "" {
				speaker = podcastspeaker.PreferredDisplayName(line.Speaker)
			}
			if speaker == "" {
				speaker = strings.TrimSpace(line.Speaker)
			}
			lines = append(lines, printLine{
				Speaker:      speaker,
				SpeakerClass: speakerClassName(line.Speaker),
				TextHTML:     renderRubyHTML(content, line.Ruby),
				Translation:  strings.TrimSpace(line.Translation),
			})
		}
		if len(lines) == 0 {
			continue
		}
		view.Sections = append(view.Sections, printSection{
			Heading: strings.TrimSpace(section.Heading),
			Lines:   lines,
		})
	}

	view.Vocabulary = buildPrintCardsFromVocabulary(vocabulary)
	view.Grammar = buildPrintCardsFromGrammar(grammar)
	return view, nil
}

func buildPrintCardsFromVocabulary(items []vocabularyItem) []printCard {
	out := make([]printCard, 0, len(items))
	for _, item := range items {
		headline := renderTokenHTML(item.Term, item.Tokens)
		if headline == "" {
			continue
		}
		out = append(out, printCard{
			HeadlineHTML: headline,
			Meaning:      strings.TrimSpace(item.Meaning),
			Explanation:  strings.TrimSpace(item.Explanation),
			Examples:     buildPrintExamples(item.Examples),
		})
	}
	return out
}

func buildPrintCardsFromGrammar(items []grammarItem) []printCard {
	out := make([]printCard, 0, len(items))
	for _, item := range items {
		headline := renderTokenHTML(item.Pattern, item.Tokens)
		if headline == "" {
			continue
		}
		out = append(out, printCard{
			HeadlineHTML: headline,
			Meaning:      strings.TrimSpace(item.Meaning),
			Explanation:  strings.TrimSpace(item.Explanation),
			Examples:     buildPrintExamples(item.Examples),
		})
	}
	return out
}

func buildPrintExamples(items []exampleDocument) []printExample {
	out := make([]printExample, 0, len(items))
	for _, item := range items {
		content := strings.TrimSpace(item.Text)
		if content == "" {
			continue
		}
		out = append(out, printExample{
			TextHTML:    renderTokenHTML(content, item.Tokens),
			Translation: strings.TrimSpace(item.Translation),
		})
	}
	return out
}

func writePrintHTML(outputPath string, view printView) error {
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return fmt.Errorf("create html output dir failed: %w", err)
	}
	var html bytes.Buffer
	if err := podcastPrintTemplate.Execute(&html, view); err != nil {
		return fmt.Errorf("render print html failed: %w", err)
	}
	if err := os.WriteFile(outputPath, html.Bytes(), 0o644); err != nil {
		return fmt.Errorf("write print html failed: %w", err)
	}
	return nil
}

func printHTMLToPDF(htmlPath, outputPath string) error {
	browserPath, err := findPDFBrowserPath()
	if err != nil {
		return err
	}

	absHTMLPath, err := filepath.Abs(htmlPath)
	if err != nil {
		return fmt.Errorf("resolve html path failed: %w", err)
	}
	absOutputPath, err := filepath.Abs(outputPath)
	if err != nil {
		return fmt.Errorf("resolve pdf path failed: %w", err)
	}

	htmlURL, err := fileURL(absHTMLPath)
	if err != nil {
		return err
	}
	profileDir, err := os.MkdirTemp("", "podcast-pdf-browser-*")
	if err != nil {
		return fmt.Errorf("create browser profile dir failed: %w", err)
	}
	defer os.RemoveAll(profileDir)

	_ = os.Remove(absOutputPath)
	args := []string{
		"--headless",
		"--disable-gpu",
		"--disable-dev-shm-usage",
		"--allow-file-access-from-files",
		"--no-pdf-header-footer",
		"--no-sandbox",
		"--run-all-compositor-stages-before-draw",
		"--virtual-time-budget=12000",
		"--user-data-dir=" + profileDir,
		"--print-to-pdf=" + absOutputPath,
		htmlURL,
	}

	cmd := exec.Command(browserPath, args...)
	cmd.Dir = filepath.Dir(absOutputPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("browser pdf render failed with %s: %w: %s", browserPath, err, strings.TrimSpace(string(output)))
	}

	info, err := os.Stat(absOutputPath)
	if err != nil {
		return fmt.Errorf("expected pdf output missing: %w", err)
	}
	if info.Size() == 0 {
		return fmt.Errorf("browser generated empty pdf: %s", absOutputPath)
	}
	return nil
}

func findPDFBrowserPath() (string, error) {
	for _, key := range []string{"WORKER_PDF_BROWSER_BIN", "PDF_BROWSER_BIN", "CHROME_BIN", "CHROMIUM_BIN"} {
		candidate := strings.TrimSpace(os.Getenv(key))
		if candidate == "" {
			continue
		}
		if filepath.IsAbs(candidate) {
			if _, err := os.Stat(candidate); err != nil {
				return "", fmt.Errorf("%s points to missing browser binary: %s", key, candidate)
			}
			return candidate, nil
		}
		resolved, err := exec.LookPath(candidate)
		if err != nil {
			return "", fmt.Errorf("%s points to missing browser binary: %s", key, candidate)
		}
		return resolved, nil
	}

	candidates := []string{
		"chromium",
		"chromium-browser",
		"google-chrome",
		"google-chrome-stable",
		"chrome",
		"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
		"/Applications/Chromium.app/Contents/MacOS/Chromium",
	}
	for _, candidate := range candidates {
		if filepath.IsAbs(candidate) {
			if _, err := os.Stat(candidate); err == nil {
				return candidate, nil
			}
			continue
		}
		if resolved, err := exec.LookPath(candidate); err == nil {
			return resolved, nil
		}
	}
	return "", fmt.Errorf("no Chromium browser found for PDF export; install chromium or set PDF_BROWSER_BIN")
}

func fileURL(path string) (string, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "", fmt.Errorf("empty file path")
	}
	absPath, err := filepath.Abs(trimmed)
	if err != nil {
		return "", fmt.Errorf("resolve file path failed: %w", err)
	}
	return (&url.URL{Scheme: "file", Path: filepath.ToSlash(absPath)}).String(), nil
}

func localizedPrintLabels(language string) printLabels {
	switch normalizedLanguage(language) {
	case "zh":
		return printLabels{
			Script:     "聊天脚本",
			Vocabulary: "词汇",
			Grammar:    "语法",
		}
	case "ja":
		return printLabels{
			Script:     "会話スクリプト",
			Vocabulary: "単語",
			Grammar:    "文法",
		}
	default:
		return printLabels{
			Script:     "Transcript",
			Vocabulary: "Vocabulary",
			Grammar:    "Grammar",
		}
	}
}

func printThemeForLanguage(language string) printTheme {
	switch normalizedLanguage(language) {
	case "zh":
		return printTheme{
			Accent:       "#b44a2e",
			AccentSoft:   "#f5e3da",
			AccentStrong: "#7f2d1b",
			Paper:        "#fcf8f3",
			Panel:        "#fffdfa",
			Border:       "#e8d8cb",
			Ink:          "#201714",
			Muted:        "#6c5d55",
		}
	case "ja":
		return printTheme{
			Accent:       "#2f638a",
			AccentSoft:   "#e2edf6",
			AccentStrong: "#1f4665",
			Paper:        "#f7fafb",
			Panel:        "#ffffff",
			Border:       "#d7e2ea",
			Ink:          "#1d252d",
			Muted:        "#61717e",
		}
	default:
		return printTheme{
			Accent:       "#2d5b4a",
			AccentSoft:   "#dfeedf",
			AccentStrong: "#1d3c31",
			Paper:        "#f8faf7",
			Panel:        "#ffffff",
			Border:       "#d8e5dc",
			Ink:          "#1e2622",
			Muted:        "#5f6b65",
		}
	}
}

func displayFontPathForLanguage(language string) string {
	switch normalizedLanguage(language) {
	case "ja":
		return resolveFontAssetPath(
			filepath.Join("fonts", "jp", "Hina-Mincho-Regular.ttf"),
			filepath.Join("fonts", "jp", "ZenKurenaido-Regular.ttf"),
		)
	case "en":
		return resolveFontAssetPath(filepath.Join("fonts", "en", "JacquesFrancois-Regular.ttf"))
	default:
		return resolveFontAssetPath(
			filepath.Join("fonts", "zh", "ruimeijiazhangqingpingyingbikaishu.ttf"),
			filepath.Join("fonts", "zh", "hanchanzhengkaiti.ttf"),
		)
	}
}

func renderTokenHTML(text string, tokens []phoneticToken) template.HTML {
	content := strings.TrimSpace(text)
	if content == "" {
		var builder strings.Builder
		for _, token := range tokens {
			builder.WriteString(token.Char)
		}
		content = strings.TrimSpace(builder.String())
	}
	if content == "" {
		return template.HTML("")
	}

	rubyTokens := make([]rubyToken, 0, len(tokens))
	for _, token := range tokens {
		rubyTokens = append(rubyTokens, rubyToken{
			Surface: strings.TrimSpace(token.Char),
			Reading: strings.TrimSpace(token.Reading),
		})
	}
	return renderRubyHTML(content, rubyTokens)
}

func renderRubyHTML(text string, tokens []rubyToken) template.HTML {
	content := strings.TrimSpace(text)
	if content == "" {
		return template.HTML("")
	}
	normalizedTokens := normalizeRubyTokens(tokens)
	if len(normalizedTokens) == 0 {
		return template.HTML(template.HTMLEscapeString(content))
	}

	var builder strings.Builder
	cursor := 0
	for _, token := range normalizedTokens {
		surface := strings.TrimSpace(token.Surface)
		reading := strings.TrimSpace(token.Reading)
		if surface == "" || reading == "" {
			continue
		}
		position := strings.Index(content[cursor:], surface)
		if position == -1 {
			continue
		}
		position += cursor
		builder.WriteString(template.HTMLEscapeString(content[cursor:position]))
		builder.WriteString("<ruby>")
		builder.WriteString(template.HTMLEscapeString(surface))
		builder.WriteString("<rt>")
		builder.WriteString(template.HTMLEscapeString(reading))
		builder.WriteString("</rt></ruby>")
		cursor = position + len(surface)
	}
	if cursor < len(content) {
		builder.WriteString(template.HTMLEscapeString(content[cursor:]))
	}
	return template.HTML(builder.String())
}

func normalizeRubyTokens(tokens []rubyToken) []rubyToken {
	out := make([]rubyToken, 0, len(tokens))
	for _, token := range tokens {
		surface := strings.TrimSpace(token.Surface)
		reading := strings.TrimSpace(token.Reading)
		if surface == "" || reading == "" {
			continue
		}

		chars := []rune(surface)
		syllables := strings.Fields(reading)
		if len(chars) > 1 && len(chars) == len(syllables) {
			for idx, char := range chars {
				out = append(out, rubyToken{
					Surface: string(char),
					Reading: syllables[idx],
				})
			}
			continue
		}
		out = append(out, rubyToken{Surface: surface, Reading: reading})
	}
	return out
}

func summaryParagraphs(summary string) []string {
	parts := strings.Split(summary, "\n\n")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func normalizedLanguage(language string) string {
	normalized := strings.ToLower(strings.TrimSpace(language))
	switch {
	case podcastspeaker.IsJapaneseLanguage(normalized):
		return "ja"
	case strings.HasPrefix(normalized, "zh"):
		return "zh"
	case strings.HasPrefix(normalized, "en"):
		return "en"
	default:
		return normalized
	}
}

func resolveFontAssetPath(fontRelPaths ...string) string {
	candidates := make([]string, 0, 10)
	if configured := strings.TrimSpace(conf.Get[string]("worker.worker_assets_dir")); configured != "" {
		candidates = append(candidates, configured)
	}
	if cwd, err := os.Getwd(); err == nil {
		for current := cwd; current != "" && current != string(filepath.Separator); current = filepath.Dir(current) {
			candidates = append(candidates, filepath.Join(current, "assets"))
			candidates = append(candidates, filepath.Join(current, "worker", "assets"))
			parent := filepath.Dir(current)
			if parent == current {
				break
			}
		}
	}

	for _, fontRelPath := range fontRelPaths {
		trimmed := strings.TrimSpace(fontRelPath)
		if trimmed == "" {
			continue
		}
		for _, baseDir := range candidates {
			fullPath := filepath.Join(baseDir, trimmed)
			if _, err := os.Stat(fullPath); err == nil {
				return fullPath
			}
		}
	}
	if len(fontRelPaths) == 0 {
		return ""
	}
	return filepath.Join("assets", strings.TrimSpace(fontRelPaths[0]))
}

func speakerClassName(role string) string {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "female", "f", "woman", "girl", "女":
		return "speaker-female"
	case "male", "m", "man", "boy", "男":
		return "speaker-male"
	default:
		return ""
	}
}
