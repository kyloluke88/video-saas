package dto

import (
	"fmt"
	"strings"
)

type PodcastAudioGeneratePayload struct {
	ProjectID       string `json:"project_id"`
	Lang            string `json:"lang"`
	ContentProfile  string `json:"content_profile"`
	IsDirect        int    `json:"is_direct,omitempty"`
	Title           string `json:"title,omitempty"`
	ScriptFilename  string `json:"script_filename"`
	BgImgFilename   string `json:"bg_img_filename"`
	TargetPlatform  string `json:"target_platform,omitempty"`
	AspectRatio     string `json:"aspect_ratio,omitempty"`
	Resolution      string `json:"resolution,omitempty"`
	DesignStyle     int    `json:"design_style,omitempty"`
	MaleVoiceType   *int64 `json:"male_voice_type,omitempty"`
	FemaleVoiceType *int64 `json:"female_voice_type,omitempty"`
}

type PodcastComposePayload struct {
	ProjectID      string `json:"project_id"`
	Lang           string `json:"lang"`
	Title          string `json:"title,omitempty"`
	BgImgFilename  string `json:"bg_img_filename"`
	TargetPlatform string `json:"target_platform,omitempty"`
	AspectRatio    string `json:"aspect_ratio,omitempty"`
	Resolution     string `json:"resolution,omitempty"`
	DesignStyle    int    `json:"design_style,omitempty"`
}

type PodcastScript struct {
	Language              string           `json:"language,omitempty"`
	AudienceLanguage      string           `json:"audience_language,omitempty"`
	DifficultyLevel       string           `json:"difficulty_level,omitempty"`
	TargetDurationMinutes int              `json:"target_duration_minutes,omitempty"`
	Title                 string           `json:"title,omitempty"`
	YouTube               PodcastYouTube   `json:"youtube,omitempty"`
	Blocks                []PodcastBlock   `json:"blocks,omitempty"`
	Segments              []PodcastSegment `json:"segments,omitempty"`
}

type PodcastBlock struct {
	MacroBlock string           `json:"macro_block,omitempty"`
	ChapterID  string           `json:"chapter_id,omitempty"`
	TTSBlockID string           `json:"tts_block_id,omitempty"`
	Purpose    string           `json:"purpose,omitempty"`
	Segments   []PodcastSegment `json:"segments,omitempty"`
}

type PodcastYouTube struct {
	PublishTitle              string                  `json:"publish_title,omitempty"`
	Chapters                  []PodcastYouTubeChapter `json:"chapters,omitempty"`
	InThisEpisodeYouWillLearn []string                `json:"in_this_episode_you_will_learn,omitempty"`
	DescriptionIntro          []string                `json:"description_intro,omitempty"`
}

type PodcastYouTubeChapter struct {
	ChapterID string   `json:"chapter_id,omitempty"`
	TitleEN   string   `json:"title_en,omitempty"`
	TitleJA   string   `json:"title_ja,omitempty"`
	TitleZH   string   `json:"title_zh,omitempty"`
	BlockIDs  []string `json:"block_ids,omitempty"`
}

type PodcastSegment struct {
	SegmentID string `json:"segment_id"`
	Speaker   string `json:"speaker,omitempty"`
	ZH        string `json:"zh,omitempty"`
	JA        string `json:"ja,omitempty"`
	DisplayJA string `json:"display_ja,omitempty"`
	TTSJA     string `json:"tts_ja,omitempty"`
	EN        string `json:"en,omitempty"`
	Summary   bool   `json:"summary,omitempty"`
	StartMS   int    `json:"start_ms,omitempty"`
	EndMS     int    `json:"end_ms,omitempty"`

	Tokens     []PodcastToken     `json:"tokens,omitempty"`
	Chars      []PodcastCharToken `json:"chars,omitempty"`
	RubyTokens []PodcastRubyToken `json:"ruby_tokens,omitempty"`
	RubySpans  []PodcastRubySpan  `json:"ruby_spans,omitempty"`
}

type PodcastToken struct {
	Char    string `json:"char"`
	Pinyin  string `json:"pinyin,omitempty"`
	StartMS int    `json:"start_ms,omitempty"`
	EndMS   int    `json:"end_ms,omitempty"`
}

type PodcastCharToken struct {
	Index   int    `json:"i"`
	Char    string `json:"c"`
	StartMS int    `json:"s,omitempty"`
	EndMS   int    `json:"e,omitempty"`
}

type PodcastRubyToken struct {
	Surface string `json:"surface"`
	Reading string `json:"reading"`
}

type PodcastRubySpan struct {
	StartIndex int    `json:"s"`
	EndIndex   int    `json:"e"`
	Ruby       string `json:"r"`
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
				TitleJA:   meta.TitleJA,
				TitleZH:   meta.TitleZH,
				BlockIDs:  make([]string, 0, 2),
			})
			chapterIndexByNewID[newChapterID] = len(chapters) - 1
		}

		s.Blocks[i].ChapterID = newChapterID
		s.Blocks[i].TTSBlockID = formatBlockID(blockIDPrefix(s.Blocks[i]), nextBlock)
		nextBlock++

		chapters[chapterIndexByNewID[newChapterID]].BlockIDs = append(
			chapters[chapterIndexByNewID[newChapterID]].BlockIDs,
			s.Blocks[i].TTSBlockID,
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
	if value := strings.TrimSpace(block.MacroBlock); value != "" {
		return value
	}
	raw := strings.TrimSpace(block.TTSBlockID)
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
