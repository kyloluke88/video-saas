package podcast_export_service

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"

	conf "worker/pkg/config"
	ffmpegcommon "worker/services/media/ffmpeg/common"
	dto "worker/services/podcast/model"
	podcastpageservice "worker/services/podcast/page"
	podcastspeaker "worker/services/podcast/speaker"
)

const (
	youtubeTranscriptFilename        = "youtube_transcript.srt"
	youtubeTranscriptEnglishFilename = "youtube_transcript_en.srt"
)

type publishVocabularyItem struct {
	Term    string `json:"term"`
	Meaning string `json:"meaning"`
}

type publishGrammarItem struct {
	Pattern string `json:"pattern"`
	Meaning string `json:"meaning"`
}

func exportYouTubeAssets(projectDir string, source podcastpageservice.PageSource) error {
	script := source.Script
	if len(script.Segments) == 0 && len(script.Blocks) > 0 {
		script.RefreshSegmentsFromBlocks()
	}

	pageURL := strings.TrimSpace(source.Upsert.CanonicalURL)
	if pageURL == "" {
		pageURL = podcastpageservice.BuildCanonicalURL(source.Upsert.Slug)
	}

	publishContent := ""
	if !isEmptyYouTubeMetadata(script.YouTube) {
		publishContent = buildYouTubePublishText(script, pageURL)
	}
	if err := writeOptionalFile(filepath.Join(projectDir, youtubePublishFilename), publishContent); err != nil {
		return err
	}

	transcriptContent := buildYouTubeTranscriptSRTWithLeadIn(script, youtubePublishLeadInMS(script.Language))
	if err := writeOptionalFile(filepath.Join(projectDir, youtubeTranscriptFilename), transcriptContent); err != nil {
		return err
	}

	englishTranscriptContent := buildYouTubeEnglishTranscriptSRTWithLeadIn(script, youtubePublishLeadInMS(script.Language))
	return writeOptionalFile(filepath.Join(projectDir, youtubeTranscriptEnglishFilename), englishTranscriptContent)
}

func writeOptionalFile(path, content string) error {
	if strings.TrimSpace(content) == "" {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

func buildYouTubePublishText(script dto.PodcastScript, pageURL string) string {
	return buildYouTubePublishTextWithLeadIn(script, pageURL, youtubePublishLeadInMS(script.Language))
}

func buildYouTubePublishTextWithLeadIn(script dto.PodcastScript, pageURL string, leadInMS int) string {
	var lines []string

	if desc := buildYouTubeDescriptionBodyLines(script); len(desc) > 0 {
		lines = append(lines, desc...)
		lines = append(lines, "")
	}

	if title := strings.TrimSpace(buildYouTubeUploadTitle(script)); title != "" {
		lines = append(lines, "Title:")
		lines = append(lines, title, "")
	}

	if hashtags := buildPublishHashtagLines(script); len(hashtags) > 0 {
		lines = append(lines, "Hashtags:")
		lines = append(lines, hashtags...)
		lines = append(lines, "")
	}

	if chapterLines := buildYouTubeChapterLinesWithLeadIn(script, leadInMS); len(chapterLines) > 0 {
		lines = append(lines, chapterLines...)
		lines = append(lines, "")
	}

	if highlights := buildEpisodeStudyHighlightLines(script); len(highlights) > 0 {
		lines = append(lines, highlights...)
		lines = append(lines, "")
	}

	lines = append(lines, buildPodcastScriptPageCTALines(pageURL)...)
	lines = append(lines, "")

	if studioTags := buildStudioTags(script); len(studioTags) > 0 {
		lines = append(lines, "Studio Tags (paste into YouTube Tags field only, comma-separated phrases are OK):")
		lines = append(lines, strings.Join(studioTags, ", "), "")
	}

	return strings.TrimSpace(strings.Join(lines, "\n")) + "\n"
}

func buildYouTubeDescriptionBodyLines(script dto.PodcastScript) []string {
	var lines []string

	lines = append(lines, compactNonEmpty(script.YouTube.DescriptionIntro)...)
	if len(lines) > 0 {
		lines = append(lines, "")
	}

	learn := compactNonEmpty(script.YouTube.InThisEpisodeYouWillLearn)
	if len(learn) > 0 {
		lines = append(lines, "In this episode, you will learn:")
		for _, item := range learn {
			lines = append(lines, "- "+item)
		}
		lines = append(lines, "")
	}

	return trimTrailingBlankLines(lines)
}

func buildYouTubeChapterLinesWithLeadIn(script dto.PodcastScript, leadInMS int) []string {
	blockStartMS := make(map[string]int, len(script.Blocks))
	chapterStartMS := make(map[string]int, len(script.Blocks))
	for _, block := range script.Blocks {
		startMS, ok := podcastBlockStartMS(block)
		if !ok {
			continue
		}
		if blockID := strings.TrimSpace(block.BlockID); blockID != "" {
			blockStartMS[blockID] = startMS
		}
		if chapterID := strings.TrimSpace(block.ChapterID); chapterID != "" {
			if current, exists := chapterStartMS[chapterID]; !exists || startMS < current {
				chapterStartMS[chapterID] = startMS
			}
		}
	}

	type chapterLine struct {
		startMS int
		title   string
	}
	items := make([]chapterLine, 0, len(script.YouTube.Chapters))
	for _, chapter := range script.YouTube.Chapters {
		startMS, ok := chapterStartMSForPublish(chapter, blockStartMS, chapterStartMS)
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
		items = append(items, chapterLine{startMS: startMS, title: title})
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].startMS == items[j].startMS {
			return items[i].title < items[j].title
		}
		return items[i].startMS < items[j].startMS
	})

	lines := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for i, item := range items {
		displayStartMS := item.startMS
		if i > 0 && leadInMS > 0 {
			displayStartMS += leadInMS
		}
		line := fmt.Sprintf("%s %s", formatTimestampMMSS(displayStartMS), item.title)
		if _, exists := seen[line]; exists {
			continue
		}
		seen[line] = struct{}{}
		lines = append(lines, line)
	}
	return lines
}

func buildEpisodeStudyHighlightLines(script dto.PodcastScript) []string {
	var vocabulary []publishVocabularyItem
	if len(script.Vocabulary) > 0 {
		_ = json.Unmarshal(script.Vocabulary, &vocabulary)
	}

	var grammar []publishGrammarItem
	if len(script.Grammar) > 0 {
		_ = json.Unmarshal(script.Grammar, &grammar)
	}

	if len(vocabulary) == 0 && len(grammar) == 0 {
		return nil
	}

	lines := []string{"Key vocabulary and grammar from this episode:"}
	if len(vocabulary) > 0 {
		lines = append(lines, "Vocabulary:")
		for _, item := range vocabulary {
			term := strings.TrimSpace(item.Term)
			meaning := strings.TrimSpace(item.Meaning)
			if term == "" {
				continue
			}
			if meaning != "" {
				lines = append(lines, fmt.Sprintf("- %s: %s", term, meaning))
				continue
			}
			lines = append(lines, "- "+term)
		}
	}
	if len(grammar) > 0 {
		lines = append(lines, "Grammar:")
		for _, item := range grammar {
			pattern := strings.TrimSpace(item.Pattern)
			meaning := strings.TrimSpace(item.Meaning)
			if pattern == "" {
				continue
			}
			if meaning != "" {
				lines = append(lines, fmt.Sprintf("- %s: %s", pattern, meaning))
				continue
			}
			lines = append(lines, "- "+pattern)
		}
	}
	return trimTrailingBlankLines(lines)
}

func buildPodcastScriptPageCTALines(pageURL string) []string {
	pageURL = strings.TrimSpace(pageURL)
	if pageURL == "" {
		pageURL = podcastpageservice.BuildCanonicalURL("")
	}
	return []string{
		"Read the full podcast script and download the PDF study sheet here:",
		pageURL,
	}
}

func chapterStartMSForPublish(chapter dto.PodcastYouTubeChapter, blockStartMS, chapterStartMS map[string]int) (int, bool) {
	startMS := 0
	found := false
	for _, blockID := range chapter.BlockIDs {
		blockID = strings.TrimSpace(blockID)
		if blockID == "" {
			continue
		}
		value, ok := blockStartMS[blockID]
		if !ok {
			continue
		}
		if !found || value < startMS {
			startMS = value
			found = true
		}
	}
	if found {
		return startMS, true
	}
	if chapterID := strings.TrimSpace(chapter.ChapterID); chapterID != "" {
		value, ok := chapterStartMS[chapterID]
		if ok {
			return value, true
		}
	}
	return 0, false
}

func podcastBlockStartMS(block dto.PodcastBlock) (int, bool) {
	for _, seg := range block.Segments {
		if seg.EndMS > seg.StartMS {
			return seg.StartMS, true
		}
	}
	return 0, false
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

func compactNonEmpty(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		out = append(out, value)
	}
	return out
}

func compactHashtags(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if !strings.HasPrefix(value, "#") {
			value = "#" + value
		}
		key := strings.ToLower(value)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, value)
	}
	return out
}

func buildPublishHashtagLines(script dto.PodcastScript) []string {
	values := buildPublishHashtags(script)
	if len(values) == 0 {
		return nil
	}

	const perLine = 5
	lines := make([]string, 0, (len(values)+perLine-1)/perLine)
	for i := 0; i < len(values); i += perLine {
		end := i + perLine
		if end > len(values) {
			end = len(values)
		}
		lines = append(lines, strings.Join(values[i:end], " "))
	}
	return lines
}

func buildPublishHashtags(script dto.PodcastScript) []string {
	base := sanitizeHashtagsForLanguage(script.YouTube.Hashtags, script.Language)
	if len(base) == 0 {
		base = languageDefaultHashtags(script)
	}

	out := make([]string, 0, 10)
	seen := make(map[string]struct{}, 16)
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		key := strings.ToLower(value)
		if _, exists := seen[key]; exists {
			return
		}
		seen[key] = struct{}{}
		out = append(out, value)
	}

	for _, tag := range base {
		add(tag)
	}
	for _, tag := range hashtagsFromVideoTags(sanitizeStudioTagsForLanguage(script.YouTube.VideoTags, script.Language), out) {
		add(tag)
	}
	for _, tag := range languageDefaultHashtags(script) {
		if len(out) >= 10 {
			break
		}
		add(tag)
	}
	if len(out) > 10 {
		out = out[:10]
	}
	return out
}

func hashtagsFromVideoTags(values, existing []string) []string {
	seen := make(map[string]struct{}, len(values))
	for _, value := range existing {
		seen[strings.ToLower(strings.TrimSpace(value))] = struct{}{}
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		tag := normalizeToHashtag(value)
		if tag == "" {
			continue
		}
		key := strings.ToLower(tag)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, tag)
	}
	return out
}

func normalizeToHashtag(value string) string {
	value = strings.TrimSpace(strings.TrimPrefix(value, "#"))
	if value == "" {
		return ""
	}

	parts := hashtagWordPattern.FindAllString(value, -1)
	if len(parts) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("#")
	for _, part := range parts {
		lower := strings.ToLower(part)
		switch {
		case strings.HasPrefix(lower, "hsk") && len(part) > 3:
			b.WriteString("HSK")
			b.WriteString(part[3:])
		case strings.EqualFold(part, "hsk"):
			b.WriteString("HSK")
		default:
			rs := []rune(lower)
			if len(rs) == 0 {
				continue
			}
			b.WriteRune(unicode.ToUpper(rs[0]))
			if len(rs) > 1 {
				b.WriteString(string(rs[1:]))
			}
		}
	}
	if b.Len() == 1 {
		return ""
	}
	return b.String()
}

func buildYouTubeUploadTitle(script dto.PodcastScript) string {
	englishTitle, chineseFromPublish := splitBilingualTitle(script.YouTube.PublishTitle)
	chineseTitle := normalizeChineseTopicTitle(firstNonEmptyString(chineseFromPublish, script.Title))

	parts := make([]string, 0, 4)
	if difficulty := normalizeDifficultyLabel(script); difficulty != "" {
		parts = append(parts, difficulty)
	}
	if englishTitle != "" {
		parts = append(parts, englishTitle)
	}
	if chineseTitle != "" {
		parts = append(parts, chineseTitle)
	}
	if suffix := languageChannelSuffix(script.Language); suffix != "" {
		parts = append(parts, suffix)
	}
	return strings.Join(parts, " | ")
}

func splitBilingualTitle(value string) (string, string) {
	parts := strings.Split(value, "|")
	if len(parts) == 0 {
		return "", ""
	}
	english := strings.TrimSpace(parts[0])
	chinese := ""
	if len(parts) > 1 {
		chinese = strings.TrimSpace(parts[1])
	}
	return english, chinese
}

func normalizeChineseTopicTitle(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	for _, sep := range []string{"：", ":"} {
		if idx := strings.Index(value, sep); idx > 0 {
			value = strings.TrimSpace(value[:idx])
			break
		}
	}
	if strings.HasPrefix(value, "现在") && len([]rune(value)) > len([]rune("现在"))+2 {
		value = strings.TrimSpace(strings.TrimPrefix(value, "现在"))
	}
	return strings.TrimSpace(value)
}

func normalizeDifficultyLabel(script dto.PodcastScript) string {
	candidates := []string{
		script.DifficultyLevel,
		strings.Join(script.YouTube.DescriptionIntro, " "),
		strings.Join(script.YouTube.Hashtags, " "),
		strings.Join(script.YouTube.VideoTags, " "),
		script.Title,
		script.YouTube.PublishTitle,
	}
	for _, candidate := range candidates {
		if label := extractDifficultyLabel(candidate); label != "" {
			return label
		}
	}
	return ""
}

func extractDifficultyLabel(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if matches := difficultyRangePattern.FindStringSubmatch(value); len(matches) == 3 {
		return fmt.Sprintf("HSK %s - %s", matches[1], matches[2])
	}
	if matches := difficultySinglePattern.FindStringSubmatch(value); len(matches) == 2 {
		return fmt.Sprintf("HSK %s", matches[1])
	}
	return ""
}

func buildStudioTags(script dto.PodcastScript) []string {
	base := sanitizeStudioTagsForLanguage(script.YouTube.VideoTags, script.Language)
	defaults := languageDefaultStudioTags(script)
	out := make([]string, 0, len(base)+len(defaults)+8)
	seen := make(map[string]struct{}, len(base)+len(defaults)+8)
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		key := strings.ToLower(value)
		if _, exists := seen[key]; exists {
			return
		}
		seen[key] = struct{}{}
		out = append(out, value)
	}

	for _, tag := range base {
		add(tag)
	}
	for _, tag := range defaults {
		add(tag)
	}

	englishTitle, chineseTitle := splitBilingualTitle(script.YouTube.PublishTitle)
	titleBlob := strings.ToLower(strings.TrimSpace(englishTitle + " " + chineseTitle + " " + script.Title))
	if isChineseLanguage(script.Language) && (strings.Contains(titleBlob, "married") || strings.Contains(titleBlob, "marriage") || strings.Contains(titleBlob, "婚恋") || strings.Contains(titleBlob, "结婚")) {
		add("marriage in china")
		add("chinese relationship")
		add("婚恋观")
		add("结婚")
		add("催婚")
		add("相亲")
	}
	if podcastspeaker.IsJapaneseLanguage(script.Language) && (strings.Contains(titleBlob, "oshikatsu") || strings.Contains(titleBlob, "oshi") || strings.Contains(titleBlob, "推し")) {
		add("oshikatsu")
		add("oshi culture")
		add("japanese fandom culture")
		add("推し活")
		add("推し文化")
	}
	if spacedHSKTag := spacedHSKStudioTag(base); spacedHSKTag != "" {
		add(spacedHSKTag)
	}
	return out
}

func spacedHSKStudioTag(values []string) string {
	for _, value := range values {
		matches := difficultySinglePattern.FindStringSubmatch(value)
		if len(matches) == 2 {
			return fmt.Sprintf("hsk %s chinese", matches[1])
		}
	}
	return ""
}

func sanitizeHashtagsForLanguage(values []string, language string) []string {
	out := make([]string, 0, len(values))
	for _, value := range compactHashtags(values) {
		if tagMatchesLanguage(value, language) {
			out = append(out, value)
		}
	}
	return out
}

func sanitizeStudioTagsForLanguage(values []string, language string) []string {
	out := make([]string, 0, len(values))
	for _, value := range compactNonEmpty(values) {
		if tagMatchesLanguage(value, language) {
			out = append(out, value)
		}
	}
	return out
}

func tagMatchesLanguage(value, language string) bool {
	value = strings.TrimSpace(strings.ToLower(strings.TrimPrefix(value, "#")))
	if value == "" {
		return false
	}
	if isChineseLanguage(language) {
		if containsJapaneseKana(value) || containsAny(value, chineseBlockedTagTerms...) {
			return false
		}
		return true
	}
	if podcastspeaker.IsJapaneseLanguage(language) {
		if containsAny(value, japaneseBlockedTagTerms...) {
			return false
		}
		return true
	}
	return true
}

func containsAny(value string, blocked ...string) bool {
	for _, part := range blocked {
		if strings.Contains(value, part) {
			return true
		}
	}
	return false
}

func containsJapaneseKana(value string) bool {
	for _, r := range value {
		if unicode.In(r, unicode.Hiragana, unicode.Katakana) {
			return true
		}
	}
	return false
}

func languageDefaultHashtags(script dto.PodcastScript) []string {
	if podcastspeaker.IsJapaneseLanguage(script.Language) {
		return []string{
			"#StudyJapanese",
			"#JapaneseListening",
			"#LearnJapanese",
			"#JapanesePodcast",
			"#NaturalJapanese",
			"#JapaneseListeningPractice",
			"#EverydayJapanese",
			"#JapaneseConversation",
			"#JapaneseCulture",
			"#DailyJapanese",
		}
	}
	return []string{
		"#StudyChinese",
		"#ChineseListening",
		"#HSK3",
		"#ChinesePodcast",
		"#LearnChinese",
		"#ChineseListeningPractice",
		"#HSK3Chinese",
		"#ChineseConversation",
		"#MandarinListening",
		"#DailyChinese",
	}
}

func languageDefaultStudioTags(script dto.PodcastScript) []string {
	if podcastspeaker.IsJapaneseLanguage(script.Language) {
		return []string{
			"learn japanese",
			"japanese listening practice",
			"japanese podcast",
			"natural japanese",
			"everyday japanese",
			"japanese conversation",
			"study japanese",
			"japanese listening",
			"日本語",
			"日本語勉強",
			"日本語リスニング",
			"日本語ポッドキャスト",
		}
	}
	return []string{
		"learn chinese",
		"chinese listening practice",
		"chinese podcast",
		"mandarin podcast",
		"study chinese",
		"learn mandarin",
		"chinese conversation",
		"中文",
		"学中文",
		"中文听力",
		"中文播客",
		"汉语",
		"汉语听力",
	}
}

func languageChannelSuffix(language string) string {
	if podcastspeaker.IsJapaneseLanguage(language) {
		return "Japanese Daily Podcast"
	}
	if isChineseLanguage(language) {
		return "Chinese Daily Podcast"
	}
	return ""
}

func trimTrailingBlankLines(lines []string) []string {
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func isChineseLanguage(language string) bool {
	switch strings.ToLower(strings.TrimSpace(language)) {
	case "zh", "zh-cn", "zh-hans":
		return true
	default:
		return false
	}
}

func isEmptyYouTubeMetadata(meta dto.PodcastYouTube) bool {
	return strings.TrimSpace(meta.PublishTitle) == "" &&
		len(compactNonEmpty(meta.InThisEpisodeYouWillLearn)) == 0 &&
		len(compactNonEmpty(meta.DescriptionIntro)) == 0 &&
		len(compactHashtags(meta.Hashtags)) == 0 &&
		len(compactNonEmpty(meta.VideoTags)) == 0 &&
		len(meta.Chapters) == 0
}

func buildYouTubeTranscriptSRT(script dto.PodcastScript) string {
	return buildYouTubeTranscriptSRTWithLeadIn(script, 0)
}

func buildYouTubeTranscriptSRTWithLeadIn(script dto.PodcastScript, leadInMS int) string {
	segments := transcriptSegments(script)
	if len(segments) == 0 {
		return ""
	}

	var b strings.Builder
	index := 1
	for _, seg := range segments {
		if seg.EndMS <= seg.StartMS {
			continue
		}
		text := transcriptCueText(script.Language, seg)
		if text == "" {
			continue
		}
		b.WriteString(fmt.Sprintf(
			"%d\n%s --> %s\n%s\n\n",
			index,
			formatSRTTimestampMS(seg.StartMS+leadInMS),
			formatSRTTimestampMS(seg.EndMS+leadInMS),
			text,
		))
		index++
	}
	return strings.TrimSpace(b.String()) + "\n"
}

func buildYouTubeEnglishTranscriptSRT(script dto.PodcastScript) string {
	return buildYouTubeEnglishTranscriptSRTWithLeadIn(script, 0)
}

func buildYouTubeEnglishTranscriptSRTWithLeadIn(script dto.PodcastScript, leadInMS int) string {
	segments := englishTranscriptSegments(script)
	if len(segments) == 0 {
		return ""
	}

	var b strings.Builder
	index := 1
	for _, seg := range segments {
		if seg.EndMS <= seg.StartMS {
			continue
		}
		text := englishTranscriptCueText(seg)
		if text == "" {
			continue
		}
		b.WriteString(fmt.Sprintf(
			"%d\n%s --> %s\n%s\n\n",
			index,
			formatSRTTimestampMS(seg.StartMS+leadInMS),
			formatSRTTimestampMS(seg.EndMS+leadInMS),
			text,
		))
		index++
	}
	return strings.TrimSpace(b.String()) + "\n"
}

func transcriptSegments(script dto.PodcastScript) []dto.PodcastSegment {
	if len(script.Segments) == 0 && len(script.Blocks) > 0 {
		script.RefreshSegmentsFromBlocks()
	}
	if len(script.Segments) == 0 {
		return nil
	}
	out := make([]dto.PodcastSegment, 0, len(script.Segments))
	for _, seg := range script.Segments {
		if seg.EndMS <= seg.StartMS {
			continue
		}
		if strings.TrimSpace(transcriptCueText(script.Language, seg)) == "" {
			continue
		}
		out = append(out, seg)
	}
	return out
}

func transcriptCueText(language string, seg dto.PodcastSegment) string {
	text := youtubeTranscriptDisplayText(language, seg)
	if text == "" {
		return ""
	}
	return strings.Join(strings.Fields(text), " ")
}

func youtubeTranscriptDisplayText(language string, seg dto.PodcastSegment) string {
	if podcastspeaker.IsJapaneseLanguage(language) {
		return strings.TrimSpace(seg.Text)
	}
	return strings.TrimSpace(seg.Text)
}

func englishTranscriptSegments(script dto.PodcastScript) []dto.PodcastSegment {
	if len(script.Segments) == 0 && len(script.Blocks) > 0 {
		script.RefreshSegmentsFromBlocks()
	}
	if len(script.Segments) == 0 {
		return nil
	}
	out := make([]dto.PodcastSegment, 0, len(script.Segments))
	for _, seg := range script.Segments {
		if seg.EndMS <= seg.StartMS {
			continue
		}
		if strings.TrimSpace(seg.EnglishTranslation()) == "" {
			continue
		}
		out = append(out, seg)
	}
	return out
}

func englishTranscriptCueText(seg dto.PodcastSegment) string {
	return strings.Join(strings.Fields(strings.TrimSpace(seg.EnglishTranslation())), " ")
}

func formatSRTTimestampMS(ms int) string {
	if ms < 0 {
		ms = 0
	}
	hours := ms / 3600000
	ms -= hours * 3600000
	minutes := ms / 60000
	ms -= minutes * 60000
	seconds := ms / 1000
	millis := ms % 1000
	return fmt.Sprintf("%02d:%02d:%02d,%03d", hours, minutes, seconds, millis)
}

func youtubePublishLeadInMS(language string) int {
	introPath := youtubeIntroAnimationPath(language)
	if strings.TrimSpace(introPath) == "" {
		return 0
	}
	durationSec, err := ffmpegcommon.AudioDurationSec(introPath)
	if err != nil || durationSec <= 0 {
		return 0
	}
	return int(durationSec * 1000)
}

func youtubeIntroAnimationPath(language string) string {
	language = strings.TrimSpace(strings.ToLower(language))
	if language != "zh" && language != "ja" {
		return ""
	}
	candidates := []string{
		filepath.Join(conf.Get[string]("worker.worker_assets_dir"), "podcast", "animation", language+"_open.mp4"),
		filepath.Join("assets", "podcast", "animation", language+"_open.mp4"),
		filepath.Join("worker", "assets", "podcast", "animation", language+"_open.mp4"),
		filepath.Join("/Users/luca/go/github.com/luca/video-saas/worker/assets/podcast/animation", language+"_open.mp4"),
	}
	for _, candidate := range candidates {
		if fileExists(candidate) {
			return candidate
		}
	}
	return ""
}

func fileExists(path string) bool {
	path = strings.TrimSpace(path)
	if path == "" {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

var (
	hashtagWordPattern      = regexp.MustCompile(`[A-Za-z0-9]+`)
	difficultyRangePattern  = regexp.MustCompile(`(?i)HSK\s*([1-6])\s*[-–]\s*(?:HSK\s*)?([1-6])`)
	difficultySinglePattern = regexp.MustCompile(`(?i)HSK\s*([1-6])`)
	chineseBlockedTagTerms  = []string{"japanese", "nihongo", "jlpt", "日本語", "日语", "日文", "にほんご", "推し活"}
	japaneseBlockedTagTerms = []string{"chinese", "mandarin", "hsk", "中文", "汉语", "漢語", "学中文", "中文听力", "中文播客"}
)
