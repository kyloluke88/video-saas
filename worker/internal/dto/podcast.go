package dto

type PodcastAudioGeneratePayload struct {
	ProjectID       string `json:"project_id"`
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
	Summary   string   `json:"summary,omitempty"`
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
