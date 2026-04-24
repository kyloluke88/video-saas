package practical_page_service

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"worker/internal/persistence"
	conf "worker/pkg/config"
	services "worker/services"
	dto "worker/services/practical/model"
)

const youtubePublishFilename = "youtube_publish.txt"

type PersistInput struct {
	ProjectID string
}

type PersistResult struct {
	PageID                uint64
	Slug                  string
	YouTubePublishPath    string
	YouTubeTranscriptPath []string
}

type PageSource struct {
	ProjectDir string
	RunMode    int
	Script     dto.PracticalScript
	Upsert     persistence.PracticalScriptPageUpsert
}

type requestPayload struct {
	Lang                string   `json:"lang"`
	ScriptFilename      string   `json:"script_filename"`
	RunMode             int      `json:"run_mode,omitempty"`
	SourceProjectID     string   `json:"source_project_id,omitempty"`
	BgImgFilenames      []string `json:"bg_img_filenames,omitempty"`
	BlockBgImgFilenames []string `json:"block_bg_img_filenames,omitempty"`
	Resolution          string   `json:"resolution,omitempty"`
	DesignType          int      `json:"design_type,omitempty"`
}

type youtubeMetadata struct {
	PublishTitle              string                 `json:"publish_title"`
	Chapters                  []youtubeChapter       `json:"chapters"`
	InThisEpisodeYouWillLearn []string               `json:"in_this_episode_you_will_learn"`
	Hashtags                  []string               `json:"hashtags"`
	VideoTags                 []string               `json:"video_tags"`
	DescriptionIntro          []string               `json:"description_intro"`
	Extra                     map[string]interface{} `json:"-"`
}

type youtubeChapter struct {
	BlockID string `json:"block_id"`
	TitleEN string `json:"title_en"`
	Title   string `json:"title"`
}

type srtCue struct {
	StartMS int
	EndMS   int
	Text    string
}

func Persist(input PersistInput) (PersistResult, error) {
	source, err := BuildPageSource(input)
	if err != nil {
		return PersistResult{}, err
	}
	return PersistSource(source)
}

func PersistSource(source PageSource) (PersistResult, error) {
	store, err := persistence.DefaultStore()
	if err != nil {
		return PersistResult{}, err
	}

	pageID, err := store.UpsertPracticalScriptPage(source.Upsert)
	if err != nil {
		return PersistResult{}, err
	}

	transcriptPaths, err := generateYouTubeTranscripts(source.ProjectDir, source.Script)
	if err != nil {
		return PersistResult{}, err
	}

	publishPath, err := generateYouTubePublishText(source.ProjectDir, source.Script)
	if err != nil {
		return PersistResult{}, err
	}

	return PersistResult{
		PageID:                pageID,
		Slug:                  source.Upsert.Slug,
		YouTubePublishPath:    publishPath,
		YouTubeTranscriptPath: transcriptPaths,
	}, nil
}

func BuildPageSource(input PersistInput) (PageSource, error) {
	projectDir := filepath.Join(conf.Get[string]("worker.ffmpeg_work_dir"), "projects", strings.TrimSpace(input.ProjectID))
	return BuildPageSourceFromProjectDir(projectDir, input)
}

func BuildPageSourceFromProjectDir(projectDir string, input PersistInput) (PageSource, error) {
	if strings.TrimSpace(input.ProjectID) == "" {
		return PageSource{}, fmt.Errorf("project_id is required")
	}
	script, err := loadScript(projectDir)
	if err != nil {
		return PageSource{}, err
	}
	if err := script.Validate(); err != nil {
		return PageSource{}, err
	}
	request, err := loadRequestPayload(projectDir)
	if err != nil {
		return PageSource{}, err
	}

	slug := buildPageSlug(script, input.ProjectID)
	if slug == "" {
		return PageSource{}, services.NonRetryableError{Err: fmt.Errorf("practical page slug is required")}
	}

	rawScript, err := json.Marshal(script)
	if err != nil {
		return PageSource{}, err
	}

	upsert := persistence.PracticalScriptPageUpsert{
		Slug:               slug,
		ProjectID:          strings.TrimSpace(input.ProjectID),
		Language:           coalesce(script.Language, request.Lang),
		AudienceLanguage:   strings.TrimSpace(script.AudienceLanguage),
		Title:              coalesce(script.Title, strings.TrimSuffix(filepath.Base(request.ScriptFilename), filepath.Ext(request.ScriptFilename)), input.ProjectID),
		EnTitle:            strings.TrimSpace(script.EnTitle),
		Subtitle:           strings.TrimSpace(script.Subtitle),
		Summary:            strings.TrimSpace(script.Summary),
		TranslationLocales: collectTranslationLocales(script),
		SEOTitle:           buildSEOTitle(script),
		SEODescription:     buildSEODescription(script),
		SEOKeywords:        buildSEOKeywords(script),
		CanonicalURL:       "",
		Script:             rawScript,
		Vocabulary:         defaultJSONArray(script.Vocabulary),
		Grammar:            defaultJSONArray(script.Grammar),
		Downloads:          json.RawMessage(`[]`),
		Status:             "published",
		PublishedAt:        nowPtr(),
	}

	return PageSource{
		ProjectDir: projectDir,
		RunMode:    request.RunMode,
		Script:     script,
		Upsert:     upsert,
	}, nil
}

func loadScript(projectDir string) (dto.PracticalScript, error) {
	candidates := []string{
		filepath.Join(projectDir, "script_aligned.json"),
		filepath.Join(projectDir, "script_input.json"),
	}

	var lastErr error
	for _, candidate := range candidates {
		raw, err := os.ReadFile(candidate)
		if err != nil {
			lastErr = err
			continue
		}
		var script dto.PracticalScript
		if err := json.Unmarshal(raw, &script); err != nil {
			return dto.PracticalScript{}, fmt.Errorf("decode %s failed: %w", candidate, err)
		}
		script.Normalize()
		return script, nil
	}
	return dto.PracticalScript{}, fmt.Errorf("load practical script failed: %w", lastErr)
}

func loadRequestPayload(projectDir string) (requestPayload, error) {
	raw, err := os.ReadFile(filepath.Join(projectDir, "request_payload.json"))
	if err != nil {
		return requestPayload{}, fmt.Errorf("read request_payload.json failed: %w", err)
	}
	var payload requestPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return requestPayload{}, fmt.Errorf("decode request_payload.json failed: %w", err)
	}
	return payload, nil
}

func collectTranslationLocales(script dto.PracticalScript) []string {
	if locales := compactStrings(script.TranslationLocales); len(locales) > 0 {
		return locales
	}
	return compactStrings(script.DetectTranslationLocales())
}

func buildSEOTitle(script dto.PracticalScript) string {
	if value := strings.TrimSpace(script.SEOTitle); value != "" {
		return value
	}
	meta := parseYouTubeMetadata(script.YouTube)
	if value := strings.TrimSpace(meta.PublishTitle); value != "" {
		return value
	}
	if value := strings.TrimSpace(script.EnTitle); value != "" {
		return value
	}
	return strings.TrimSpace(script.Title)
}

func buildSEODescription(script dto.PracticalScript) string {
	if value := strings.TrimSpace(script.SEODescription); value != "" {
		return value
	}
	if value := strings.TrimSpace(script.Summary); value != "" {
		return value
	}
	meta := parseYouTubeMetadata(script.YouTube)
	lines := compactStrings(meta.DescriptionIntro)
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n\n")
}

func buildSEOKeywords(script dto.PracticalScript) []string {
	if len(script.SEOKeywords) > 0 {
		return compactStrings(script.SEOKeywords)
	}
	meta := parseYouTubeMetadata(script.YouTube)
	seen := make(map[string]struct{})
	out := make([]string, 0, len(meta.VideoTags)+len(meta.Hashtags)+2)
	appendKeyword := func(value string) {
		value = strings.TrimSpace(strings.TrimPrefix(value, "#"))
		if value == "" {
			return
		}
		if _, exists := seen[value]; exists {
			return
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	appendKeyword(script.Title)
	appendKeyword(script.EnTitle)
	for _, item := range meta.VideoTags {
		appendKeyword(item)
	}
	for _, item := range meta.Hashtags {
		appendKeyword(item)
	}
	return out
}

func buildPageSlug(script dto.PracticalScript, projectID string) string {
	base := slugify(script.EnTitle)
	if base == "" {
		base = slugify(script.Title)
	}
	if base == "" {
		base = slugify(projectID)
	}
	language := strings.ToLower(strings.TrimSpace(script.Language))
	if language == "" {
		return base
	}
	if base == "" {
		return language
	}
	return language + "-" + base
}

func generateYouTubePublishText(projectDir string, script dto.PracticalScript) (string, error) {
	content := buildYouTubePublishText(script)
	path := filepath.Join(projectDir, youtubePublishFilename)
	if strings.TrimSpace(content) == "" {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return "", err
		}
		return "", nil
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return "", err
	}
	return path, nil
}

func buildYouTubePublishText(script dto.PracticalScript) string {
	meta := parseYouTubeMetadata(script.YouTube)
	if isEmptyYouTubeMetadata(meta) {
		return ""
	}

	lines := make([]string, 0, 32)
	lines = append(lines, compactStrings(meta.DescriptionIntro)...)
	if len(lines) > 0 {
		lines = append(lines, "")
	}
	if title := strings.TrimSpace(meta.PublishTitle); title != "" {
		lines = append(lines, "Title:")
		lines = append(lines, title, "")
	}
	if learn := compactStrings(meta.InThisEpisodeYouWillLearn); len(learn) > 0 {
		lines = append(lines, "In this episode, you will learn:")
		for _, item := range learn {
			lines = append(lines, "- "+item)
		}
		lines = append(lines, "")
	}
	if hashtags := compactStrings(meta.Hashtags); len(hashtags) > 0 {
		lines = append(lines, "Hashtags:")
		lines = append(lines, strings.Join(hashtags, " "))
		lines = append(lines, "")
	}
	if chapters := buildYouTubeChapterLines(script, meta); len(chapters) > 0 {
		lines = append(lines, "Chapters:")
		lines = append(lines, chapters...)
		lines = append(lines, "")
	}
	if tags := compactStrings(meta.VideoTags); len(tags) > 0 {
		lines = append(lines, "Tags:")
		lines = append(lines, strings.Join(tags, ", "))
		lines = append(lines, "")
	}

	return strings.TrimSpace(strings.Join(trimTrailingBlankLines(lines), "\n")) + "\n"
}

func buildYouTubeChapterLines(script dto.PracticalScript, meta youtubeMetadata) []string {
	if len(meta.Chapters) == 0 {
		return nil
	}
	starts := make(map[string]int, len(script.Blocks))
	for _, block := range script.Blocks {
		if strings.TrimSpace(block.BlockID) == "" || block.StartMS < 0 {
			continue
		}
		starts[strings.TrimSpace(block.BlockID)] = block.StartMS
	}
	type chapterLine struct {
		StartMS int
		Title   string
	}
	items := make([]chapterLine, 0, len(meta.Chapters))
	for _, chapter := range meta.Chapters {
		startMS, ok := starts[strings.TrimSpace(chapter.BlockID)]
		if !ok {
			continue
		}
		title := strings.TrimSpace(chapter.TitleEN)
		if title == "" {
			title = strings.TrimSpace(chapter.Title)
		}
		if title == "" {
			continue
		}
		items = append(items, chapterLine{StartMS: startMS, Title: title})
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].StartMS == items[j].StartMS {
			return items[i].Title < items[j].Title
		}
		return items[i].StartMS < items[j].StartMS
	})
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, fmt.Sprintf("%s %s", formatTimestampMMSS(item.StartMS), item.Title))
	}
	return out
}

func generateYouTubeTranscripts(projectDir string, script dto.PracticalScript) ([]string, error) {
	existing, err := filepath.Glob(filepath.Join(projectDir, "youtube_transcript_*.srt"))
	if err != nil {
		return nil, err
	}
	for _, path := range existing {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return nil, err
		}
	}

	locales := collectTranslationLocales(script)
	if len(locales) == 0 {
		return nil, nil
	}

	turns := flattenTurns(script)
	paths := make([]string, 0, len(locales))
	for _, locale := range locales {
		cues := make([]srtCue, 0, len(turns))
		for _, turn := range turns {
			if turn.EndMS <= turn.StartMS {
				continue
			}
			text := strings.TrimSpace(turn.TranslationFor(locale))
			if text == "" {
				continue
			}
			cues = append(cues, srtCue{
				StartMS: turn.StartMS,
				EndMS:   turn.EndMS,
				Text:    text,
			})
		}
		if len(cues) == 0 {
			continue
		}
		path := filepath.Join(projectDir, fmt.Sprintf("youtube_transcript_%s.srt", strings.TrimSpace(locale)))
		if err := writeSRTFile(path, cues); err != nil {
			return nil, err
		}
		paths = append(paths, path)
	}
	return paths, nil
}

func flattenTurns(script dto.PracticalScript) []dto.PracticalTurn {
	out := make([]dto.PracticalTurn, 0)
	for _, block := range script.Blocks {
		for _, chapter := range block.Chapters {
			out = append(out, chapter.Turns...)
		}
	}
	return out
}

func writeSRTFile(path string, cues []srtCue) error {
	if len(cues) == 0 {
		return nil
	}
	var b strings.Builder
	index := 1
	for _, cue := range cues {
		text := strings.TrimSpace(cue.Text)
		if text == "" {
			continue
		}
		lines := splitSRTLines(text, 48, 2)
		if len(lines) == 0 {
			continue
		}
		b.WriteString(strconv.Itoa(index))
		b.WriteString("\n")
		b.WriteString(formatSRTTimestampMS(cue.StartMS))
		b.WriteString(" --> ")
		b.WriteString(formatSRTTimestampMS(cue.EndMS))
		b.WriteString("\n")
		b.WriteString(strings.Join(lines, "\n"))
		b.WriteString("\n\n")
		index++
	}
	if b.Len() == 0 {
		return nil
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

func splitSRTLines(text string, maxChars, maxLines int) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	if maxChars <= 0 {
		maxChars = 48
	}
	if maxLines <= 0 {
		maxLines = 2
	}

	normalized := normalizeSpacing(text)
	if runeCount(normalized) <= maxChars || maxLines == 1 {
		return []string{normalized}
	}

	words := strings.Fields(normalized)
	if len(words) <= 1 {
		return splitRunes(normalized, maxChars, maxLines)
	}

	lines := make([]string, 0, maxLines)
	current := ""
	for _, word := range words {
		candidate := word
		if current != "" {
			candidate = current + " " + word
		}
		if runeCount(candidate) > maxChars && current != "" && len(lines) < maxLines-1 {
			lines = append(lines, current)
			current = word
			continue
		}
		current = candidate
	}
	if current != "" {
		lines = append(lines, current)
	}
	if len(lines) > maxLines {
		tail := strings.Join(lines[maxLines-1:], " ")
		lines = append(lines[:maxLines-1], tail)
	}
	return lines
}

func splitRunes(text string, maxChars, maxLines int) []string {
	runes := []rune(text)
	if len(runes) <= maxChars || maxLines <= 1 {
		return []string{text}
	}
	lines := make([]string, 0, maxLines)
	for len(runes) > 0 && len(lines) < maxLines-1 {
		take := minInt(maxChars, len(runes))
		lines = append(lines, strings.TrimSpace(string(runes[:take])))
		runes = runes[take:]
	}
	if len(runes) > 0 {
		lines = append(lines, strings.TrimSpace(string(runes)))
	}
	return lines
}

func normalizeSpacing(text string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(text)), " ")
}

func runeCount(text string) int {
	return len([]rune(text))
}

func formatSRTTimestampMS(ms int) string {
	if ms < 0 {
		ms = 0
	}
	hours := ms / 3600000
	ms %= 3600000
	minutes := ms / 60000
	ms %= 60000
	seconds := ms / 1000
	millis := ms % 1000
	return fmt.Sprintf("%02d:%02d:%02d,%03d", hours, minutes, seconds, millis)
}

func formatTimestampMMSS(ms int) string {
	if ms < 0 {
		ms = 0
	}
	totalSeconds := ms / 1000
	minutes := totalSeconds / 60
	seconds := totalSeconds % 60
	return fmt.Sprintf("%02d:%02d", minutes, seconds)
}

func parseYouTubeMetadata(raw json.RawMessage) youtubeMetadata {
	if len(raw) == 0 {
		return youtubeMetadata{}
	}
	var meta youtubeMetadata
	_ = json.Unmarshal(raw, &meta)
	return meta
}

func isEmptyYouTubeMetadata(meta youtubeMetadata) bool {
	return strings.TrimSpace(meta.PublishTitle) == "" &&
		len(compactStrings(meta.ChaptersToStrings())) == 0 &&
		len(compactStrings(meta.InThisEpisodeYouWillLearn)) == 0 &&
		len(compactStrings(meta.Hashtags)) == 0 &&
		len(compactStrings(meta.VideoTags)) == 0 &&
		len(compactStrings(meta.DescriptionIntro)) == 0
}

func (m youtubeMetadata) ChaptersToStrings() []string {
	out := make([]string, 0, len(m.Chapters))
	for _, chapter := range m.Chapters {
		if text := strings.TrimSpace(chapter.BlockID + chapter.TitleEN + chapter.Title); text != "" {
			out = append(out, text)
		}
	}
	return out
}

func compactStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func trimTrailingBlankLines(values []string) []string {
	end := len(values)
	for end > 0 && strings.TrimSpace(values[end-1]) == "" {
		end--
	}
	return values[:end]
}

func coalesce(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func nowPtr() *time.Time {
	now := time.Now().UTC()
	return &now
}

func defaultJSONArray(value json.RawMessage) json.RawMessage {
	if trimmed := strings.TrimSpace(string(value)); trimmed != "" && trimmed != "null" {
		return value
	}
	return json.RawMessage(`[]`)
}

func slugify(value string) string {
	trimmed := strings.TrimSpace(strings.ToLower(value))
	if trimmed == "" {
		return ""
	}

	var builder strings.Builder
	lastDash := false
	for _, r := range trimmed {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
			lastDash = false
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
			lastDash = false
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			builder.WriteRune(r)
			lastDash = false
		default:
			if lastDash || builder.Len() == 0 {
				continue
			}
			builder.WriteRune('-')
			lastDash = true
		}
	}

	result := strings.Trim(builder.String(), "-")
	re := regexp.MustCompile(`-+`)
	result = re.ReplaceAllString(result, "-")
	if len(result) > 120 {
		result = strings.Trim(result[:120], "-")
	}
	return result
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
