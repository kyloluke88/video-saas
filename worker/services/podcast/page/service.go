package podcast_page_service

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode"

	"worker/internal/persistence"
	conf "worker/pkg/config"
	services "worker/services"
	dto "worker/services/podcast/model"
	podcastspeaker "worker/services/podcast/speaker"
)

type PersistInput struct {
	ProjectID       string
	VideoURL        string
	YouTubeVideoID  string
	YouTubeVideoURL string
}

type PersistResult struct {
	PageID uint64
	Slug   string
}

type PageSource struct {
	ProjectDir string
	Script     dto.PodcastScript
	Upsert     persistence.ScriptPageUpsert
}

type requestPayload struct {
	Lang           string   `json:"lang"`
	Title          string   `json:"title"`
	ScriptFilename string   `json:"script_filename"`
	TargetPlatform string   `json:"target_platform"`
	BgImgFilenames []string `json:"bg_img_filenames"`
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
	Note        string      `json:"note,omitempty"`
}

type rubyToken struct {
	Surface string `json:"surface"`
	Reading string `json:"reading"`
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

	pageID, err := store.UpsertPodcastScriptPage(source.Upsert)
	if err != nil {
		return PersistResult{}, err
	}

	return PersistResult{
		PageID: pageID,
		Slug:   source.Upsert.Slug,
	}, nil
}

func BuildPageSource(input PersistInput) (PageSource, error) {
	projectDir := filepath.Join(conf.Get[string]("worker.ffmpeg_work_dir"), "projects", input.ProjectID)
	return BuildPageSourceFromProjectDir(projectDir, input)
}

func BuildPageUpsert(input PersistInput) (persistence.ScriptPageUpsert, error) {
	source, err := BuildPageSource(input)
	if err != nil {
		return persistence.ScriptPageUpsert{}, err
	}
	return source.Upsert, nil
}

func BuildPageUpsertFromProjectDir(projectDir string, input PersistInput) (persistence.ScriptPageUpsert, error) {
	source, err := BuildPageSourceFromProjectDir(projectDir, input)
	if err != nil {
		return persistence.ScriptPageUpsert{}, err
	}
	return source.Upsert, nil
}

func BuildPageSourceFromProjectDir(projectDir string, input PersistInput) (PageSource, error) {
	if strings.TrimSpace(input.ProjectID) == "" {
		return PageSource{}, fmt.Errorf("project_id is required")
	}
	script, err := loadScript(projectDir)
	if err != nil {
		return PageSource{}, err
	}
	request, err := loadRequestPayload(projectDir)
	if err != nil {
		return PageSource{}, err
	}
	slug := buildPageSlug(script)
	if slug == "" {
		return PageSource{}, services.NonRetryableError{Err: fmt.Errorf("script en_title is required for slug generation")}
	}

	if len(script.Segments) == 0 {
		script.RefreshSegmentsFromBlocks()
	}

	youtubeVideoID := strings.TrimSpace(input.YouTubeVideoID)
	youtubeVideoURL := strings.TrimSpace(input.YouTubeVideoURL)
	if youtubeVideoID == "" {
		youtubeVideoID = extractYouTubeVideoID(youtubeVideoURL)
	}

	scriptJSON, err := json.Marshal(buildScriptDocument(script))
	if err != nil {
		return PageSource{}, err
	}

	upsert := persistence.ScriptPageUpsert{
		Slug:             slug,
		ProjectID:        input.ProjectID,
		Language:         coalesce(script.Language, request.Lang),
		AudienceLanguage: "en",
		Title:            coalesce(script.Title, request.Title, strings.TrimSuffix(filepath.Base(request.ScriptFilename), filepath.Ext(request.ScriptFilename)), input.ProjectID),
		EnTitle:          strings.TrimSpace(script.EnTitle),
		Subtitle:         buildSubtitle(script),
		Summary:          buildSummary(script),
		VideoURL:         strings.TrimSpace(input.VideoURL),
		YouTubeVideoID:   youtubeVideoID,
		YouTubeVideoURL:  youtubeVideoURL,
		SEOTitle:         coalesce(script.YouTube.PublishTitle, script.Title),
		SEODescription:   buildSEODescription(script),
		SEOKeywords:      buildKeywords(script),
		CanonicalURL:     BuildCanonicalURL(slug),
		Script:           scriptJSON,
		Vocabulary:       script.Vocabulary,
		Grammar:          script.Grammar,
		Status:           "published",
		PublishedAt:      nowPtr(),
	}

	return PageSource{
		ProjectDir: projectDir,
		Script:     script,
		Upsert:     upsert,
	}, nil
}

func loadScript(projectDir string) (dto.PodcastScript, error) {
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
		var script dto.PodcastScript
		if err := json.Unmarshal(raw, &script); err != nil {
			return dto.PodcastScript{}, fmt.Errorf("decode %s failed: %w", candidate, err)
		}
		return script, nil
	}
	return dto.PodcastScript{}, fmt.Errorf("load podcast script failed: %w", lastErr)
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

func buildScriptDocument(script dto.PodcastScript) scriptDocument {
	doc := scriptDocument{
		Sections: buildSections(script),
	}
	if doc.Sections == nil {
		doc.Sections = []sectionDocument{}
	}
	return doc
}

func buildSections(script dto.PodcastScript) []sectionDocument {
	if len(script.Blocks) == 0 {
		return buildSingleSection(script)
	}

	blocksByChapter := make(map[string][]dto.PodcastBlock, len(script.Blocks))
	for _, block := range script.Blocks {
		chapterID := strings.TrimSpace(block.ChapterID)
		if chapterID == "" {
			chapterID = "default"
		}
		blocksByChapter[chapterID] = append(blocksByChapter[chapterID], block)
	}

	sections := make([]sectionDocument, 0, len(script.YouTube.Chapters))
	if len(script.YouTube.Chapters) > 0 {
		for _, chapter := range script.YouTube.Chapters {
			chapterID := strings.TrimSpace(chapter.ChapterID)
			blocks := blocksByChapter[chapterID]
			if len(blocks) == 0 {
				continue
			}
			sections = append(sections, sectionDocument{
				Heading: coalesce(chapter.Title, chapter.TitleEN),
				Lines:   buildSectionLines(script.Language, blocks),
			})
			delete(blocksByChapter, chapterID)
		}
	}

	for chapterID, blocks := range blocksByChapter {
		sections = append(sections, sectionDocument{
			Heading: chapterID,
			Lines:   buildSectionLines(script.Language, blocks),
		})
	}
	return sections
}

func buildSingleSection(script dto.PodcastScript) []sectionDocument {
	lines := make([]lineDocument, 0, len(script.Segments))
	for _, seg := range script.Segments {
		line := buildLine(script.Language, seg)
		if line.Text == "" {
			continue
		}
		lines = append(lines, line)
	}
	if len(lines) == 0 {
		return nil
	}
	return []sectionDocument{{
		Heading: script.Title,
		Lines:   lines,
	}}
}

func buildSectionLines(language string, blocks []dto.PodcastBlock) []lineDocument {
	lines := make([]lineDocument, 0)
	for _, block := range blocks {
		for _, seg := range block.Segments {
			line := buildLine(language, seg)
			if line.Text == "" {
				continue
			}
			lines = append(lines, line)
		}
	}
	return lines
}

func buildLine(language string, seg dto.PodcastSegment) lineDocument {
	text := buildDisplayText(language, seg)
	return lineDocument{
		Speaker:     normalizeSpeaker(seg.Speaker),
		SpeakerName: podcastspeaker.PreferredDisplayName(seg.SpeakerName),
		Text:        text,
		Ruby:        buildRuby(language, seg, text),
		Translation: strings.TrimSpace(seg.EN),
	}
}

func buildDisplayText(language string, seg dto.PodcastSegment) string {
	if isJapaneseLanguage(language) {
		return strings.TrimSpace(seg.Text)
	}
	return strings.TrimSpace(seg.Text)
}

func buildRuby(language string, seg dto.PodcastSegment, text string) []rubyToken {
	if text == "" {
		return nil
	}
	if isJapaneseLanguage(language) {
		return buildJapaneseRuby(seg, text)
	}
	return buildChineseRuby(seg)
}

func buildChineseRuby(seg dto.PodcastSegment) []rubyToken {
	out := make([]rubyToken, 0, len(seg.Tokens))
	for _, token := range seg.Tokens {
		surface := strings.TrimSpace(token.Char)
		reading := strings.TrimSpace(token.Reading)
		if surface == "" || reading == "" || isSilentToken(surface) {
			continue
		}
		out = append(out, rubyToken{Surface: surface, Reading: reading})
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func buildJapaneseRuby(seg dto.PodcastSegment, text string) []rubyToken {
	spans := seg.TokenSpans
	if len(spans) == 0 {
		spans = dto.BuildJapaneseTokenSpans(text, seg.Tokens)
	}
	if len(spans) == 0 {
		return nil
	}
	runes := []rune(text)
	out := make([]rubyToken, 0, len(spans))
	for _, span := range spans {
		if span.StartIndex < 0 || span.EndIndex < span.StartIndex || span.EndIndex >= len(runes) {
			continue
		}
		surface := strings.TrimSpace(string(runes[span.StartIndex : span.EndIndex+1]))
		reading := strings.TrimSpace(span.Reading)
		if surface == "" || reading == "" {
			continue
		}
		out = append(out, rubyToken{Surface: surface, Reading: reading})
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func buildSummary(script dto.PodcastScript) string {
	parts := buildSummaryParagraphs(script)
	if len(parts) > 0 {
		return strings.Join(parts, "\n\n")
	}
	return strings.TrimSpace(script.Title)
}

func buildSEODescription(script dto.PodcastScript) string {
	parts := buildSummaryParagraphs(script)
	if len(parts) > 0 {
		return strings.Join(parts, " ")
	}
	return strings.TrimSpace(script.Title)
}

func buildSummaryParagraphs(script dto.PodcastScript) []string {
	parts := make([]string, 0, len(script.YouTube.DescriptionIntro))
	for _, item := range script.YouTube.DescriptionIntro {
		item = strings.TrimSpace(item)
		if item != "" {
			parts = append(parts, item)
		}
	}
	return parts
}

func buildSubtitle(script dto.PodcastScript) string {
	parts := make([]string, 0, 2)
	if level := strings.TrimSpace(script.DifficultyLevel); level != "" {
		parts = append(parts, level)
	}
	switch strings.ToLower(strings.TrimSpace(script.Language)) {
	case "ja", "ja-jp":
		parts = append(parts, "Japanese podcast")
	case "zh", "zh-cn":
		parts = append(parts, "Chinese podcast")
	}
	return strings.Join(parts, " · ")
}

func buildKeywords(script dto.PodcastScript) []string {
	seen := make(map[string]struct{})
	out := make([]string, 0, len(script.YouTube.VideoTags)+len(script.YouTube.Hashtags))
	for _, item := range append(script.YouTube.VideoTags, script.YouTube.Hashtags...) {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, exists := seen[item]; exists {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	return out
}

func buildPageSlug(script dto.PodcastScript) string {
	return slugify(script.EnTitle)
}

func BuildCanonicalURL(slug string) string {
	const baseURL = "https://podcast.lucayo.com"
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return baseURL
	}
	return fmt.Sprintf("%s/podcast/scripts/%s", baseURL, slug)
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
		case unicode.IsSpace(r) || r == '-' || r == '_' || r == '|' || unicode.IsPunct(r) || unicode.IsSymbol(r):
			if !lastDash && builder.Len() > 0 {
				builder.WriteRune('-')
				lastDash = true
			}
		}
	}
	result := strings.Trim(builder.String(), "-")
	if len(result) > 120 {
		result = strings.Trim(result[:120], "-")
	}
	return result
}

func extractYouTubeVideoID(rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return ""
	}
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?:v=)([A-Za-z0-9_-]{11})`),
		regexp.MustCompile(`(?:youtu\.be/)([A-Za-z0-9_-]{11})`),
		regexp.MustCompile(`(?:embed/)([A-Za-z0-9_-]{11})`),
	}
	for _, pattern := range patterns {
		matches := pattern.FindStringSubmatch(rawURL)
		if len(matches) == 2 {
			return matches[1]
		}
	}
	return ""
}

func normalizeSpeaker(value string) string {
	return podcastspeaker.NormalizeRole(value)
}

func isJapaneseLanguage(language string) bool {
	return podcastspeaker.IsJapaneseLanguage(language)
}

func isSilentToken(charText string) bool {
	rs := []rune(strings.TrimSpace(charText))
	if len(rs) != 1 {
		return false
	}
	return strings.ContainsRune("，。！？；：“”‘’（）《》、…,.!?;:()[]{}\"'", rs[0])
}

func coalesce(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func nowPtr() *time.Time {
	now := time.Now().UTC()
	return &now
}
