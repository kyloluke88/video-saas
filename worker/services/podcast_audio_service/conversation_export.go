package podcast_audio_service

import (
	"fmt"
	"path/filepath"
	"strings"

	"worker/internal/dto"
)

type conversationExport struct {
	ConversationID   string                   `json:"conversation_id"`
	Language         string                   `json:"language"`
	AudienceLanguage string                   `json:"audience_language"`
	Title            string                   `json:"title,omitempty"`
	Turns            []conversationExportTurn `json:"turns"`
}

type conversationExportTurn struct {
	TurnID      string                      `json:"turn_id"`
	Role        string                      `json:"role"`
	Speaker     string                      `json:"speaker"`
	SpeakerName string                      `json:"speaker_name"`
	Segments    []conversationExportSegment `json:"segments"`
}

type conversationExportSegment struct {
	SegmentID   string                   `json:"segment_id"`
	DisplayText string                   `json:"display_text"`
	English     string                   `json:"english,omitempty"`
	Ruby        []conversationExportRuby `json:"ruby,omitempty"`
}

type conversationExportRuby struct {
	Surface string `json:"surface"`
	Reading string `json:"reading"`
}

func exportConversationMinimalFile(projectDir, projectID, contentProfile string, script dto.PodcastScript) error {
	export := conversationExport{
		ConversationID:   strings.TrimSpace(projectID),
		Language:         strings.TrimSpace(script.Language),
		AudienceLanguage: defaultAudienceLanguage(script.AudienceLanguage),
		Title:            strings.TrimSpace(script.Title),
		Turns:            buildConversationTurns(script, contentProfile),
	}
	return writeJSON(filepath.Join(projectDir, "conversation_minimal.json"), export)
}

func buildConversationTurns(script dto.PodcastScript, contentProfile string) []conversationExportTurn {
	segments := script.Segments
	if len(segments) == 0 && len(script.Blocks) > 0 {
		script.RefreshSegmentsFromBlocks()
		segments = script.Segments
	}

	turns := make([]conversationExportTurn, 0)
	for _, seg := range segments {
		displayText := strings.TrimSpace(exportDisplayText(script.Language, seg))
		if displayText == "" {
			continue
		}

		speaker := defaultSpeaker(seg.Speaker)
		exportSeg := conversationExportSegment{
			SegmentID:   strings.TrimSpace(seg.SegmentID),
			DisplayText: displayText,
			English:     strings.TrimSpace(seg.EN),
			Ruby:        exportRuby(script.Language, seg, displayText),
		}

		if len(turns) == 0 || turns[len(turns)-1].Speaker != speaker {
			turns = append(turns, conversationExportTurn{
				TurnID:      fmt.Sprintf("turn_%03d", len(turns)+1),
				Role:        "assistant",
				Speaker:     speaker,
				SpeakerName: speakerDisplayName(script.Language, contentProfile, speaker),
				Segments:    []conversationExportSegment{exportSeg},
			})
			continue
		}

		turns[len(turns)-1].Segments = append(turns[len(turns)-1].Segments, exportSeg)
	}
	return turns
}

func exportDisplayText(language string, seg dto.PodcastSegment) string {
	if isJapaneseLanguage(language) {
		return japaneseDisplayText(seg)
	}
	return strings.TrimSpace(seg.ZH)
}

func exportRuby(language string, seg dto.PodcastSegment, displayText string) []conversationExportRuby {
	if isJapaneseLanguage(language) {
		return exportJapaneseRuby(seg, displayText)
	}
	return exportChineseRuby(seg)
}

func exportChineseRuby(seg dto.PodcastSegment) []conversationExportRuby {
	out := make([]conversationExportRuby, 0, len(seg.Tokens))
	for _, token := range seg.Tokens {
		surface := strings.TrimSpace(token.Char)
		reading := strings.TrimSpace(token.Pinyin)
		if surface == "" || reading == "" || isSilentToken(surface) {
			continue
		}
		out = append(out, conversationExportRuby{
			Surface: surface,
			Reading: reading,
		})
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func exportJapaneseRuby(seg dto.PodcastSegment, displayText string) []conversationExportRuby {
	if len(seg.RubySpans) == 0 {
		return nil
	}
	runes := []rune(displayText)
	out := make([]conversationExportRuby, 0, len(seg.RubySpans))
	for _, span := range seg.RubySpans {
		if span.StartIndex < 0 || span.EndIndex < span.StartIndex || span.EndIndex >= len(runes) {
			continue
		}
		surface := strings.TrimSpace(string(runes[span.StartIndex : span.EndIndex+1]))
		reading := strings.TrimSpace(span.Ruby)
		if surface == "" || reading == "" {
			continue
		}
		out = append(out, conversationExportRuby{
			Surface: surface,
			Reading: reading,
		})
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func speakerDisplayName(language, contentProfile, speaker string) string {
	speaker = defaultSpeaker(speaker)
	if isJapaneseLanguage(language) {
		if speaker == "female" {
			return "Yui"
		}
		return "Akira"
	}
	if speaker == "female" {
		if normalizeContentProfile(contentProfile) == "daily" || strings.TrimSpace(contentProfile) == "" {
			return "小静"
		}
		return "小赵"
	}
	return "小路"
}

func defaultAudienceLanguage(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "en"
	}
	return value
}
