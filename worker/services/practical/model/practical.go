package model

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

type PracticalAudioGeneratePayload struct {
	ProjectID           string   `json:"project_id"`
	SourceProjectID     string   `json:"source_project_id,omitempty"`
	Lang                string   `json:"lang"`
	TTSType             int      `json:"tts_type,omitempty"`
	RunMode             int      `json:"run_mode"`
	SpecifyTasks        []string `json:"specify_tasks,omitempty"`
	BlockNums           []int    `json:"block_nums,omitempty"`
	ScriptFilename      string   `json:"script_filename"`
	BgImgFilenames      []string `json:"bg_img_filenames,omitempty"`
	BlockBgImgFilenames []string `json:"block_bg_img_filenames,omitempty"`
	Resolution          string   `json:"resolution,omitempty"`
	AspectRatio         string   `json:"aspect_ratio,omitempty"`
	DesignType          int      `json:"design_type,omitempty"`
}

type PracticalScript struct {
	SchemaVersion      string           `json:"schema_version,omitempty"`
	SeriesID           string           `json:"series_id,omitempty"`
	EpisodeID          string           `json:"episode_id,omitempty"`
	Language           string           `json:"language,omitempty"`
	AudienceLanguage   string           `json:"audience_language,omitempty"`
	DifficultyLevel    string           `json:"difficulty_level,omitempty"`
	Title              string           `json:"title,omitempty"`
	EnTitle            string           `json:"en_title,omitempty"`
	Subtitle           string           `json:"subtitle,omitempty"`
	Summary            string           `json:"summary,omitempty"`
	SEOTitle           string           `json:"seo_title,omitempty"`
	SEODescription     string           `json:"seo_description,omitempty"`
	SEOKeywords        []string         `json:"seo_keywords,omitempty"`
	YouTube            json.RawMessage  `json:"youtube,omitempty"`
	Vocabulary         json.RawMessage  `json:"vocabulary,omitempty"`
	Grammar            json.RawMessage  `json:"grammar,omitempty"`
	TranslationLocales []string         `json:"translation_locales,omitempty"`
	Blocks             []PracticalBlock `json:"blocks,omitempty"`
}

type PracticalSpeaker struct {
	// SpeakerID is the runtime voice channel. Supported values are male/female.
	SpeakerID string `json:"speaker_id,omitempty"`
	// SpeakerRole is the business role identifier used by turns.
	// Example: customer, clerk, doctor.
	SpeakerRole string `json:"speaker_role,omitempty"`
	Name        string `json:"name,omitempty"`
}

type PracticalBlock struct {
	BlockID           string            `json:"block_id,omitempty"`
	Topic             string            `json:"topic,omitempty"`
	BlockPrompt       string            `json:"block_prompt,omitempty"`
	TopicTranslations map[string]string `json:"topic_translations,omitempty"`
	// Speakers are block-local. Keep their order stable so TTS voice mapping stays consistent
	// within the block even when chapter scene prompts change.
	Speakers     []PracticalSpeaker `json:"speakers,omitempty"`
	Chapters     []PracticalChapter `json:"chapters,omitempty"`
	TopicStartMS int                `json:"topic_start_ms,omitempty"`
	TopicEndMS   int                `json:"topic_end_ms,omitempty"`
	StartMS      int                `json:"start_ms,omitempty"`
	EndMS        int                `json:"end_ms,omitempty"`
}

type PracticalChapter struct {
	ChapterID         string            `json:"chapter_id,omitempty"`
	Scene             string            `json:"scene,omitempty"`
	SceneTranslations map[string]string `json:"scene_translations,omitempty"`
	ScenePrompt       string            `json:"scene_prompt,omitempty"`
	Turns             []PracticalTurn   `json:"turns,omitempty"`
	StartMS           int               `json:"start_ms,omitempty"`
	EndMS             int               `json:"end_ms,omitempty"`
}

type PracticalTurn struct {
	TurnID       string            `json:"turn_id,omitempty"`
	SpeakerRole  string            `json:"speaker_role,omitempty"`
	SpeakerID    string            `json:"speaker_id,omitempty"`
	Text         string            `json:"text,omitempty"`
	SpeechText   string            `json:"speech_text,omitempty"`
	Translations map[string]string `json:"translations,omitempty"`
	Tokens       json.RawMessage   `json:"tokens,omitempty"`
	StartMS      int               `json:"start_ms,omitempty"`
	EndMS        int               `json:"end_ms,omitempty"`
}

func (s *PracticalScript) Normalize() {
	s.SchemaVersion = strings.TrimSpace(s.SchemaVersion)
	s.SeriesID = strings.TrimSpace(s.SeriesID)
	s.EpisodeID = strings.TrimSpace(s.EpisodeID)
	s.Language = strings.ToLower(strings.TrimSpace(s.Language))
	s.AudienceLanguage = strings.TrimSpace(s.AudienceLanguage)
	s.DifficultyLevel = strings.TrimSpace(s.DifficultyLevel)
	s.Title = strings.TrimSpace(s.Title)
	s.EnTitle = strings.TrimSpace(s.EnTitle)
	s.Subtitle = strings.TrimSpace(s.Subtitle)
	s.Summary = strings.TrimSpace(s.Summary)
	s.SEOTitle = strings.TrimSpace(s.SEOTitle)
	s.SEODescription = strings.TrimSpace(s.SEODescription)
	s.SEOKeywords = compactPracticalStrings(s.SEOKeywords)

	for i := range s.Blocks {
		s.Blocks[i].Normalize()
	}
	s.TranslationLocales = compactPracticalStrings(s.TranslationLocales)
	if len(s.TranslationLocales) == 0 {
		s.TranslationLocales = s.DetectTranslationLocales()
	}
}

func (s *PracticalScript) Validate() error {
	s.Normalize()
	if !validPracticalLanguage(s.Language) {
		return fmt.Errorf("lang must be zh or ja")
	}
	if len(s.Blocks) == 0 {
		return fmt.Errorf("practical script requires non-empty blocks")
	}

	for _, block := range s.Blocks {
		if strings.TrimSpace(block.BlockID) == "" {
			return fmt.Errorf("practical block requires block_id")
		}
		if strings.TrimSpace(block.Topic) == "" {
			return fmt.Errorf("practical block %s topic is required", block.BlockID)
		}
		speakerVoicesByRole, err := block.SpeakerVoicesByRole()
		if err != nil {
			return fmt.Errorf("practical block %s: %w", block.BlockID, err)
		}
		if len(block.Chapters) == 0 {
			return fmt.Errorf("practical block %s has no chapters", block.BlockID)
		}
		for _, chapter := range block.Chapters {
			if strings.TrimSpace(chapter.ChapterID) == "" {
				return fmt.Errorf("practical chapter requires chapter_id")
			}
			if len(chapter.Turns) == 0 {
				return fmt.Errorf("practical chapter %s has no turns", chapter.ChapterID)
			}
			for _, turn := range chapter.Turns {
				if strings.TrimSpace(turn.TurnID) == "" {
					return fmt.Errorf("practical turn_id is required")
				}
				key := firstPracticalNonEmpty(turn.SpeakerRole, turn.SpeakerID)
				if key == "" {
					return fmt.Errorf("turn %s speaker_role is required", turn.TurnID)
				}
				turnVoice := normalizePracticalSpeakerVoice(turn.SpeakerID)
				if strings.TrimSpace(turn.SpeakerRole) != "" {
					mappedVoice, ok := speakerVoicesByRole[strings.TrimSpace(turn.SpeakerRole)]
					if !ok {
						return fmt.Errorf("turn %s speaker_role %s is not declared in speakers", turn.TurnID, strings.TrimSpace(turn.SpeakerRole))
					}
					if turnVoice != "" && turnVoice != mappedVoice {
						return fmt.Errorf("turn %s speaker_id %s conflicts with role %s voice %s", turn.TurnID, strings.TrimSpace(turn.SpeakerID), strings.TrimSpace(turn.SpeakerRole), mappedVoice)
					}
				} else if turnVoice == "" {
					if _, ok := speakerVoicesByRole[strings.TrimSpace(turn.SpeakerID)]; !ok {
						return fmt.Errorf("turn %s speaker reference %s is not declared in speakers", turn.TurnID, strings.TrimSpace(turn.SpeakerID))
					}
				}
				if strings.TrimSpace(turn.Text) == "" {
					return fmt.Errorf("turn %s text is required", turn.TurnID)
				}
			}
		}
	}
	return nil
}

func (b *PracticalBlock) SpeakerVoicesByRole() (map[string]string, error) {
	if len(b.Speakers) < 2 {
		return nil, fmt.Errorf("practical block requires at least 2 speakers")
	}
	out := make(map[string]string, len(b.Speakers))
	hasMale := false
	hasFemale := false
	for _, speaker := range b.Speakers {
		role := strings.TrimSpace(speaker.SpeakerRole)
		if role == "" {
			return nil, fmt.Errorf("speaker_role is required")
		}
		if _, exists := out[role]; exists {
			return nil, fmt.Errorf("duplicate speaker_role %s", role)
		}
		voice := normalizePracticalSpeakerVoice(speaker.SpeakerID)
		if voice == "" {
			return nil, fmt.Errorf("speaker %s requires speaker_id male/female", role)
		}
		if voice == "male" {
			hasMale = true
		}
		if voice == "female" {
			hasFemale = true
		}
		out[role] = voice
	}
	if !hasMale || !hasFemale {
		return nil, fmt.Errorf("practical block requires at least one male and one female speaker for google tts")
	}
	return out, nil
}

func (b *PracticalBlock) ResolveTurnVoice(turn PracticalTurn) (string, error) {
	role := strings.TrimSpace(turn.SpeakerRole)
	voice := normalizePracticalSpeakerVoice(turn.SpeakerID)
	voicesByRole, err := b.SpeakerVoicesByRole()
	if err != nil {
		return "", err
	}
	if role != "" {
		if mapped, ok := voicesByRole[role]; ok {
			if voice == "" || voice == mapped {
				return mapped, nil
			}
			return "", fmt.Errorf("turn %s speaker_id %s conflicts with role %s voice %s", strings.TrimSpace(turn.TurnID), strings.TrimSpace(turn.SpeakerID), role, mapped)
		}
		return "", fmt.Errorf("turn %s speaker_role %s is not declared in speakers", strings.TrimSpace(turn.TurnID), role)
	}
	if voice != "" {
		return voice, nil
	}
	key := strings.TrimSpace(turn.SpeakerID)
	if key != "" {
		if mapped, ok := voicesByRole[key]; ok {
			return mapped, nil
		}
	}
	return "", fmt.Errorf("turn %s speaker_role is required", strings.TrimSpace(turn.TurnID))
}

func (b *PracticalBlock) SpeakerNames() map[string]string {
	out := map[string]string{
		"female": "female",
		"male":   "male",
	}
	for _, speaker := range b.Speakers {
		voice := normalizePracticalSpeakerVoice(speaker.SpeakerID)
		if voice == "" {
			continue
		}
		if out[voice] != voice {
			continue
		}
		out[voice] = firstPracticalNonEmpty(speaker.Name, speaker.SpeakerRole, speaker.SpeakerID, voice)
	}
	return out
}

func (b *PracticalBlock) Normalize() {
	b.BlockID = strings.TrimSpace(b.BlockID)
	b.Topic = strings.TrimSpace(b.Topic)
	b.BlockPrompt = strings.TrimSpace(b.BlockPrompt)
	b.TopicTranslations = normalizePracticalTranslations(b.TopicTranslations)
	b.Speakers = normalizePracticalSpeakers(b.Speakers)
	for i := range b.Chapters {
		b.Chapters[i].Normalize()
	}
}

func (c *PracticalChapter) Normalize() {
	c.ChapterID = strings.TrimSpace(c.ChapterID)
	c.Scene = strings.TrimSpace(c.Scene)
	c.ScenePrompt = strings.TrimSpace(c.ScenePrompt)
	c.SceneTranslations = normalizePracticalTranslations(c.SceneTranslations)
	for i := range c.Turns {
		c.Turns[i].Normalize()
	}
}

func (t *PracticalTurn) Normalize() {
	t.TurnID = strings.TrimSpace(t.TurnID)
	t.SpeakerRole = strings.TrimSpace(t.SpeakerRole)
	t.SpeakerID = strings.TrimSpace(t.SpeakerID)
	if t.SpeakerRole == "" && normalizePracticalSpeakerVoice(t.SpeakerID) == "" {
		// Backward compatibility for older scripts that used speaker_id as role.
		t.SpeakerRole = t.SpeakerID
	}
	t.Text = strings.TrimSpace(t.Text)
	t.SpeechText = firstPracticalNonEmpty(t.SpeechText, t.Text)
	t.Translations = normalizePracticalTranslations(t.Translations)
}

func (t PracticalTurn) TranslationFor(language string) string {
	language = strings.TrimSpace(language)
	if language == "" {
		return ""
	}
	return strings.TrimSpace(t.Translations[language])
}

func (s PracticalScript) DetectTranslationLocales() []string {
	seen := make(map[string]struct{})
	out := make([]string, 0, 8)
	appendLang := func(raw string) {
		lang := strings.TrimSpace(raw)
		if lang == "" {
			return
		}
		if _, ok := seen[lang]; ok {
			return
		}
		seen[lang] = struct{}{}
		out = append(out, lang)
	}
	for _, block := range s.Blocks {
		for lang := range block.TopicTranslations {
			appendLang(lang)
		}
		for _, chapter := range block.Chapters {
			for lang := range chapter.SceneTranslations {
				appendLang(lang)
			}
			for _, turn := range chapter.Turns {
				for lang := range turn.Translations {
					appendLang(lang)
				}
			}
		}
	}
	return orderPracticalLocales(out)
}

func normalizePracticalSpeakers(values []PracticalSpeaker) []PracticalSpeaker {
	out := make([]PracticalSpeaker, 0, len(values))
	for idx, speaker := range values {
		rawID := strings.TrimSpace(speaker.SpeakerID)
		speaker.SpeakerID = normalizePracticalSpeakerVoice(rawID)
		speaker.SpeakerRole = strings.TrimSpace(speaker.SpeakerRole)
		speaker.Name = strings.TrimSpace(speaker.Name)
		if speaker.SpeakerRole == "" {
			if speaker.SpeakerID != "" {
				speaker.SpeakerRole = speaker.SpeakerID
			} else {
				speaker.SpeakerRole = rawID
			}
		}
		if speaker.SpeakerRole == "" {
			continue
		}
		if speaker.SpeakerID == "" {
			// Keep backward compatibility for scripts where speakers only declared role IDs.
			if idx%2 == 0 {
				speaker.SpeakerID = "female"
			} else {
				speaker.SpeakerID = "male"
			}
		}
		out = append(out, speaker)
	}
	return out
}

func normalizePracticalTranslations(values map[string]string) map[string]string {
	out := make(map[string]string)
	for key, value := range values {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" || value == "" {
			continue
		}
		out[key] = value
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func compactPracticalStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func firstPracticalNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func normalizePracticalSpeakerVoice(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "female", "f", "woman", "girl", "女":
		return "female"
	case "male", "m", "man", "boy", "男":
		return "male"
	default:
		return ""
	}
}

func validPracticalLanguage(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "zh", "ja":
		return true
	default:
		return false
	}
}

func orderPracticalLocales(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	preferred := map[string]int{
		"en":      0,
		"es-419":  1,
		"zh-Hans": 2,
		"vi":      3,
		"ko":      4,
		"id":      5,
	}
	out := append([]string(nil), values...)
	sort.SliceStable(out, func(i, j int) bool {
		pi, iok := preferred[out[i]]
		pj, jok := preferred[out[j]]
		switch {
		case iok && jok:
			if pi != pj {
				return pi < pj
			}
		case iok != jok:
			return iok
		}
		return out[i] < out[j]
	})
	return out
}

func MustMarshalJSON(value interface{}) json.RawMessage {
	raw, _ := json.Marshal(value)
	return raw
}
