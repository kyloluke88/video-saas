package model

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode"
)

type PodcastAudioGeneratePayload struct {
	ProjectID       string   `json:"project_id"`
	SourceProjectID string   `json:"source_project_id,omitempty"`
	Lang            string   `json:"lang"`
	ContentProfile  string   `json:"content_profile"`
	TTSType         int      `json:"tts_type,omitempty"`
	Seed            int      `json:"seed,omitempty"`
	RunMode         int      `json:"run_mode,omitempty"`
	BlockNums       []int    `json:"block_nums,omitempty"`
	Title           string   `json:"title,omitempty"`
	ScriptFilename  string   `json:"script_filename"`
	BgImgFilenames  []string `json:"bg_img_filenames,omitempty"`
	TargetPlatform  string   `json:"target_platform,omitempty"`
	AspectRatio     string   `json:"aspect_ratio,omitempty"`
	Resolution      string   `json:"resolution,omitempty"`
	DesignStyle     int      `json:"design_style,omitempty"`
}

type PodcastComposePayload struct {
	ProjectID      string   `json:"project_id"`
	Lang           string   `json:"lang"`
	Title          string   `json:"title,omitempty"`
	BgImgFilenames []string `json:"bg_img_filenames,omitempty"`
	TargetPlatform string   `json:"target_platform,omitempty"`
	AspectRatio    string   `json:"aspect_ratio,omitempty"`
	Resolution     string   `json:"resolution,omitempty"`
	DesignStyle    int      `json:"design_style,omitempty"`
}

type PodcastScript struct {
	Language        string           `json:"language,omitempty"`
	DifficultyLevel string           `json:"difficulty_level,omitempty"`
	Title           string           `json:"title,omitempty"`
	EnTitle         string           `json:"en_title,omitempty"`
	YouTube         PodcastYouTube   `json:"youtube,omitempty"`
	Vocabulary      json.RawMessage  `json:"vocabulary,omitempty"`
	Grammar         json.RawMessage  `json:"grammar,omitempty"`
	Blocks          []PodcastBlock   `json:"blocks,omitempty"`
	Segments        []PodcastSegment `json:"segments,omitempty"`
}

type PodcastBlock struct {
	ChapterID string           `json:"chapter_id,omitempty"`
	BlockID   string           `json:"block_id,omitempty"`
	Purpose   string           `json:"purpose,omitempty"`
	Segments  []PodcastSegment `json:"segments,omitempty"`
}

type PodcastYouTube struct {
	PublishTitle              string                  `json:"publish_title,omitempty"`
	Chapters                  []PodcastYouTubeChapter `json:"chapters,omitempty"`
	InThisEpisodeYouWillLearn []string                `json:"in_this_episode_you_will_learn,omitempty"`
	DescriptionIntro          []string                `json:"description_intro,omitempty"`
	Hashtags                  []string                `json:"hashtags,omitempty"`
	VideoTags                 []string                `json:"video_tags,omitempty"`
}

type PodcastYouTubeChapter struct {
	ChapterID string   `json:"chapter_id,omitempty"`
	TitleEN   string   `json:"title_en,omitempty"`
	Title     string   `json:"title,omitempty"`
	BlockIDs  []string `json:"block_ids,omitempty"`
}

type PodcastSegment struct {
	SegmentID   string `json:"segment_id"`
	Speaker     string `json:"speaker,omitempty"`
	SpeakerName string `json:"speaker_name,omitempty"`
	Text        string `json:"text,omitempty"`
	SpeechText  string `json:"speech_text,omitempty"`
	EN          string `json:"en,omitempty"`
	Summary     bool   `json:"summary,omitempty"`
	StartMS     int    `json:"start_ms,omitempty"`
	EndMS       int    `json:"end_ms,omitempty"`

	Tokens         []PodcastToken         `json:"tokens,omitempty"`
	HighlightSpans []PodcastHighlightSpan `json:"highlight_spans,omitempty"`
	TokenSpans     []PodcastTokenSpan     `json:"-"`
}

type PodcastToken struct {
	Char    string `json:"char"`
	Reading string `json:"reading,omitempty"`
	StartMS int    `json:"start_ms,omitempty"`
	EndMS   int    `json:"end_ms,omitempty"`
}

type PodcastTokenSpan struct {
	StartIndex int
	EndIndex   int
	Reading    string
}

type PodcastHighlightSpan struct {
	StartIndex int `json:"start_index"`
	EndIndex   int `json:"end_index"`
	StartMS    int `json:"start_ms,omitempty"`
	EndMS      int `json:"end_ms,omitempty"`
}

type PodcastTokenSpanRef struct {
	TokenIndex int
	Span       PodcastTokenSpan
}

func (s *PodcastScript) UnmarshalJSON(data []byte) error {
	type rawScript struct {
		Language        string           `json:"language"`
		DifficultyLevel string           `json:"difficulty_level"`
		Title           string           `json:"title"`
		EnTitle         string           `json:"en_title"`
		YouTube         PodcastYouTube   `json:"youtube"`
		Vocabulary      json.RawMessage  `json:"vocabulary"`
		Grammar         json.RawMessage  `json:"grammar"`
		Blocks          []PodcastBlock   `json:"blocks"`
		Segments        []PodcastSegment `json:"segments"`
	}
	var raw rawScript
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	s.Language = strings.TrimSpace(raw.Language)
	s.DifficultyLevel = strings.TrimSpace(raw.DifficultyLevel)
	s.Title = strings.TrimSpace(raw.Title)
	s.EnTitle = strings.TrimSpace(raw.EnTitle)
	s.YouTube = raw.YouTube
	s.Vocabulary = raw.Vocabulary
	s.Grammar = raw.Grammar
	s.Blocks = raw.Blocks
	s.Segments = raw.Segments
	if len(s.Segments) == 0 && len(s.Blocks) > 0 {
		s.RefreshSegmentsFromBlocks()
	}
	return nil
}

func (b *PodcastBlock) UnmarshalJSON(data []byte) error {
	type rawBlock struct {
		ChapterID string           `json:"chapter_id"`
		BlockID   string           `json:"block_id"`
		TTSBlock  string           `json:"tts_block_id"`
		Purpose   string           `json:"purpose"`
		Segments  []PodcastSegment `json:"segments"`
	}
	var raw rawBlock
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	b.ChapterID = strings.TrimSpace(raw.ChapterID)
	b.BlockID = firstNonEmpty(raw.BlockID, raw.TTSBlock)
	b.Purpose = strings.TrimSpace(raw.Purpose)
	b.Segments = raw.Segments
	return nil
}

func (c *PodcastYouTubeChapter) UnmarshalJSON(data []byte) error {
	type rawChapter struct {
		ChapterID string   `json:"chapter_id"`
		TitleEN   string   `json:"title_en"`
		Title     string   `json:"title"`
		TitleJA   string   `json:"title_ja"`
		TitleZH   string   `json:"title_zh"`
		BlockIDs  []string `json:"block_ids"`
	}
	var raw rawChapter
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	c.ChapterID = strings.TrimSpace(raw.ChapterID)
	c.TitleEN = strings.TrimSpace(raw.TitleEN)
	c.Title = firstNonEmpty(raw.Title, raw.TitleJA, raw.TitleZH)
	c.BlockIDs = raw.BlockIDs
	return nil
}

func (s *PodcastSegment) UnmarshalJSON(data []byte) error {
	type rawSegment struct {
		SegmentID      string                 `json:"segment_id"`
		Speaker        string                 `json:"speaker"`
		SpeakerName    string                 `json:"speaker_name"`
		Text           string                 `json:"text"`
		SpeechText     string                 `json:"speech_text"`
		TTSText        string                 `json:"tts_text"`
		DisplayJA      string                 `json:"display_ja"`
		EN             string                 `json:"en"`
		Summary        bool                   `json:"summary"`
		StartMS        int                    `json:"start_ms"`
		EndMS          int                    `json:"end_ms"`
		Tokens         []PodcastToken         `json:"tokens"`
		RubyTokens     []PodcastToken         `json:"ruby_tokens"`
		HighlightSpans []PodcastHighlightSpan `json:"highlight_spans"`
	}
	var raw rawSegment
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	s.SegmentID = strings.TrimSpace(raw.SegmentID)
	s.Speaker = strings.TrimSpace(raw.Speaker)
	s.SpeakerName = strings.TrimSpace(raw.SpeakerName)
	s.Text = firstNonEmpty(raw.Text, raw.DisplayJA)
	s.SpeechText = firstNonEmpty(raw.SpeechText, raw.TTSText)
	s.EN = strings.TrimSpace(raw.EN)
	s.Summary = raw.Summary
	s.StartMS = raw.StartMS
	s.EndMS = raw.EndMS
	s.Tokens = raw.Tokens
	s.HighlightSpans = raw.HighlightSpans
	if len(s.Tokens) == 0 {
		s.Tokens = raw.RubyTokens
	}
	return nil
}

func (t *PodcastToken) UnmarshalJSON(data []byte) error {
	type rawToken struct {
		Char    string `json:"char"`
		Surface string `json:"surface"`
		Text    string `json:"text"`
		Reading string `json:"reading"`
		StartMS int    `json:"start_ms"`
		EndMS   int    `json:"end_ms"`
	}
	var raw rawToken
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	t.Char = firstNonEmpty(raw.Char, raw.Surface, raw.Text)
	t.Reading = strings.TrimSpace(raw.Reading)
	t.StartMS = raw.StartMS
	t.EndMS = raw.EndMS
	return nil
}

func (s *PodcastScript) RefreshSegmentsFromBlocks() {
	if len(s.Blocks) == 0 {
		return
	}
	segments := make([]PodcastSegment, 0)
	for _, block := range s.Blocks {
		segments = append(segments, block.Segments...)
	}
	s.Segments = segments
}

func (s *PodcastScript) SyncBlocksFromSegments() {
	if len(s.Blocks) == 0 || len(s.Segments) == 0 {
		return
	}
	segmentsByID := make(map[string]PodcastSegment, len(s.Segments))
	for _, seg := range s.Segments {
		if seg.SegmentID == "" {
			continue
		}
		segmentsByID[seg.SegmentID] = seg
	}
	for i := range s.Blocks {
		for j := range s.Blocks[i].Segments {
			segID := s.Blocks[i].Segments[j].SegmentID
			if segID == "" {
				continue
			}
			updated, ok := segmentsByID[segID]
			if !ok {
				continue
			}
			s.Blocks[i].Segments[j] = updated
		}
	}
}

func (s *PodcastScript) RenumberStructureIDs() {
	if len(s.Blocks) == 0 {
		for i := range s.Segments {
			s.Segments[i].SegmentID = formatSegmentID(i + 1)
		}
		return
	}

	chapterMetaByOldID := make(map[string]PodcastYouTubeChapter, len(s.YouTube.Chapters))
	for _, chapter := range s.YouTube.Chapters {
		if id := strings.TrimSpace(chapter.ChapterID); id != "" {
			chapterMetaByOldID[id] = chapter
		}
	}

	newChapterIDByOldID := make(map[string]string, len(s.Blocks))
	chapterIndexByNewID := make(map[string]int, len(s.Blocks))
	chapters := make([]PodcastYouTubeChapter, 0, len(s.YouTube.Chapters))

	nextChapter := 1
	nextBlock := 1
	nextSegment := 1

	for i := range s.Blocks {
		oldChapterID := normalizedBlockChapterKey(s.Blocks[i], i)
		newChapterID, ok := newChapterIDByOldID[oldChapterID]
		if !ok {
			newChapterID = formatChapterID(nextChapter)
			nextChapter++
			newChapterIDByOldID[oldChapterID] = newChapterID

			meta := chapterMetaByOldID[oldChapterID]
			chapters = append(chapters, PodcastYouTubeChapter{
				ChapterID: newChapterID,
				TitleEN:   meta.TitleEN,
				Title:     meta.Title,
				BlockIDs:  make([]string, 0, 2),
			})
			chapterIndexByNewID[newChapterID] = len(chapters) - 1
		}

		s.Blocks[i].ChapterID = newChapterID
		s.Blocks[i].BlockID = formatBlockID(blockIDPrefix(s.Blocks[i]), nextBlock)
		nextBlock++

		chapters[chapterIndexByNewID[newChapterID]].BlockIDs = append(
			chapters[chapterIndexByNewID[newChapterID]].BlockIDs,
			s.Blocks[i].BlockID,
		)

		for j := range s.Blocks[i].Segments {
			s.Blocks[i].Segments[j].SegmentID = formatSegmentID(nextSegment)
			nextSegment++
		}
	}

	s.YouTube.Chapters = chapters
	s.RefreshSegmentsFromBlocks()
}

func normalizedBlockChapterKey(block PodcastBlock, index int) string {
	if value := strings.TrimSpace(block.ChapterID); value != "" {
		return value
	}
	return fmt.Sprintf("__chapter_%03d", index+1)
}

func blockIDPrefix(block PodcastBlock) string {
	raw := strings.TrimSpace(block.BlockID)
	if raw == "" {
		return "block"
	}
	if idx := strings.Index(raw, "."); idx > 0 {
		return raw[:idx]
	}
	return raw
}

func formatChapterID(index int) string {
	return fmt.Sprintf("ch_%03d", index)
}

func formatBlockID(prefix string, index int) string {
	clean := strings.TrimSpace(prefix)
	if clean == "" {
		clean = "block"
	}
	return fmt.Sprintf("%s.%d", clean, index)
}

func formatSegmentID(index int) string {
	return fmt.Sprintf("seg_%03d", index)
}

func BuildJapaneseTokenSpans(text string, tokens []PodcastToken) []PodcastTokenSpan {
	refs := BuildJapaneseTokenSpanRefs(text, tokens)
	if len(refs) == 0 {
		return nil
	}
	out := make([]PodcastTokenSpan, 0, len(refs))
	for _, ref := range refs {
		out = append(out, ref.Span)
	}
	return out
}

func BuildJapaneseTokenSpanRefs(text string, tokens []PodcastToken) []PodcastTokenSpanRef {
	runes := []rune(strings.TrimSpace(text))
	if len(runes) == 0 || len(tokens) == 0 {
		return nil
	}
	out := make([]PodcastTokenSpanRef, 0, len(tokens))
	searchFrom := 0
	for tokenIndex, token := range tokens {
		surface := strings.TrimSpace(token.Char)
		reading := strings.TrimSpace(token.Reading)
		if surface == "" || reading == "" {
			continue
		}
		start, end, ok := findJapaneseSurfaceRange(runes, []rune(surface), searchFrom)
		if !ok {
			continue
		}
		span, ok := normalizeJapaneseSpanRange(runes, PodcastTokenSpan{
			StartIndex: start,
			EndIndex:   end,
			Reading:    reading,
		})
		if !ok {
			searchFrom = end + 1
			continue
		}
		out = append(out, PodcastTokenSpanRef{
			TokenIndex: tokenIndex,
			Span:       span,
		})
		searchFrom = end + 1
	}
	return dedupeJapaneseTokenSpanRefs(out)
}

func ContainsJapaneseKanji(text string) bool {
	for _, r := range text {
		if unicode.In(r, unicode.Han) {
			return true
		}
	}
	return false
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func findJapaneseSurfaceRange(textRunes, surfaceRunes []rune, searchFrom int) (int, int, bool) {
	if len(surfaceRunes) == 0 || len(textRunes) == 0 || searchFrom >= len(textRunes) {
		return 0, 0, false
	}
	maxStart := len(textRunes) - len(surfaceRunes)
	for start := maxInt(searchFrom, 0); start <= maxStart; start++ {
		match := true
		for i := range surfaceRunes {
			if textRunes[start+i] != surfaceRunes[i] {
				match = false
				break
			}
		}
		if match {
			return start, start + len(surfaceRunes) - 1, true
		}
	}
	return 0, 0, false
}

func normalizeJapaneseSpanRange(runes []rune, span PodcastTokenSpan) (PodcastTokenSpan, bool) {
	firstHan := -1
	lastHan := -1
	for i := span.StartIndex; i <= span.EndIndex; i++ {
		if unicode.In(runes[i], unicode.Han) {
			if firstHan == -1 {
				firstHan = i
			}
			lastHan = i
		}
	}
	if firstHan == -1 {
		return PodcastTokenSpan{}, false
	}
	span.StartIndex = firstHan
	span.EndIndex = lastHan
	return span, true
}

func dedupeJapaneseTokenSpans(spans []PodcastTokenSpan) []PodcastTokenSpan {
	if len(spans) == 0 {
		return nil
	}
	out := make([]PodcastTokenSpan, 0, len(spans))
	lastEnd := -1
	for _, span := range spans {
		if span.StartIndex <= lastEnd {
			continue
		}
		out = append(out, span)
		lastEnd = span.EndIndex
	}
	return out
}

func dedupeJapaneseTokenSpanRefs(refs []PodcastTokenSpanRef) []PodcastTokenSpanRef {
	if len(refs) == 0 {
		return nil
	}
	out := make([]PodcastTokenSpanRef, 0, len(refs))
	lastEnd := -1
	for _, ref := range refs {
		if ref.Span.StartIndex <= lastEnd {
			continue
		}
		out = append(out, ref)
		lastEnd = ref.Span.EndIndex
	}
	return out
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
