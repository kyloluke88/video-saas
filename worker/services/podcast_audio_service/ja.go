package podcast_audio_service

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"worker/internal/dto"
)

type japaneseAnnotationSpan struct {
	TokenIndex int
	Span       dto.PodcastTokenSpan
}

func normalizeJapaneseSegment(seg dto.PodcastSegment) dto.PodcastSegment {
	seg.TokenSpans = dto.BuildJapaneseTokenSpans(japaneseDisplayText(seg), seg.Tokens)
	return fillUnalignedJapaneseAnnotationTokens(seg)
}

func japaneseAlignmentStats(seg dto.PodcastSegment) (int, int) {
	matched := 0
	for _, token := range seg.Tokens {
		if token.EndMS > token.StartMS {
			matched++
		}
	}
	return matched, len(seg.Tokens)
}

func japaneseCharacterCount(seg dto.PodcastSegment) int {
	return utf8.RuneCountInString(strings.TrimSpace(japaneseDisplayText(seg)))
}

func validateJapaneseScriptInput(script dto.PodcastScript) error {
	if len(script.Blocks) == 0 {
		return fmt.Errorf("japanese podcast script requires non-empty blocks")
	}
	for _, block := range script.Blocks {
		if strings.TrimSpace(block.BlockID) == "" {
			return fmt.Errorf("japanese podcast block requires block_id")
		}
		if len(block.Segments) == 0 {
			return fmt.Errorf("japanese podcast block %s has no segments", block.BlockID)
		}
		for _, seg := range block.Segments {
			if strings.TrimSpace(seg.SegmentID) == "" {
				return fmt.Errorf("japanese podcast segment_id is required")
			}
			text := japaneseDisplayText(seg)
			if text == "" {
				return fmt.Errorf("segment %s text is required", seg.SegmentID)
			}
			if dto.ContainsJapaneseKanji(text) && len(seg.Tokens) == 0 {
				return fmt.Errorf("segment %s tokens are required when text contains kanji", seg.SegmentID)
			}
			for _, token := range seg.Tokens {
				if strings.TrimSpace(token.Char) == "" {
					return fmt.Errorf("segment %s token char is required", seg.SegmentID)
				}
			}
		}
	}
	return nil
}

func fillUnalignedJapaneseAnnotationTokens(seg dto.PodcastSegment) dto.PodcastSegment {
	if len(seg.Tokens) == 0 {
		return seg
	}
	start := maxInt(0, seg.StartMS)
	end := maxInt(start+1, seg.EndMS)
	tokens := make([]dto.PodcastToken, len(seg.Tokens))
	copy(tokens, seg.Tokens)

	for i := 0; i < len(tokens); {
		if tokens[i].EndMS > tokens[i].StartMS {
			i++
			continue
		}

		j := i
		for j < len(tokens) && tokens[j].EndMS <= tokens[j].StartMS {
			j++
		}

		windowStart := start
		if i > 0 && tokens[i-1].EndMS > tokens[i-1].StartMS {
			windowStart = tokens[i-1].EndMS
		}
		windowEnd := end
		if j < len(tokens) && tokens[j].EndMS > tokens[j].StartMS {
			windowEnd = tokens[j].StartMS
		}
		if windowEnd <= windowStart {
			windowEnd = windowStart + maxInt(j-i, 1)
		}

		step := maxInt(1, (windowEnd-windowStart)/maxInt(j-i, 1))
		cursor := windowStart
		for k := i; k < j; k++ {
			tokens[k].StartMS = cursor
			if k == j-1 {
				tokens[k].EndMS = maxInt(cursor+1, windowEnd)
			} else {
				tokens[k].EndMS = maxInt(cursor+1, cursor+step)
			}
			cursor = tokens[k].EndMS
		}
		i = j
	}

	seg.Tokens = tokens
	seg.TokenSpans = dto.BuildJapaneseTokenSpans(japaneseDisplayText(seg), seg.Tokens)
	return seg
}

func buildJapaneseAnnotationSpans(seg dto.PodcastSegment) []japaneseAnnotationSpan {
	textRunes := []rune(japaneseDisplayText(seg))
	if len(textRunes) == 0 || len(seg.Tokens) == 0 {
		return nil
	}
	out := make([]japaneseAnnotationSpan, 0, len(seg.Tokens))
	searchFrom := 0
	for idx, token := range seg.Tokens {
		surface := strings.TrimSpace(token.Char)
		reading := strings.TrimSpace(token.Reading)
		if surface == "" || reading == "" {
			continue
		}
		start, end, ok := findJapaneseSurfaceRange(textRunes, []rune(surface), searchFrom)
		if !ok {
			continue
		}
		span, ok := normalizeJapaneseSpanRange(textRunes, dto.PodcastTokenSpan{
			StartIndex: start,
			EndIndex:   end,
			Reading:    reading,
		})
		if !ok {
			searchFrom = end + 1
			continue
		}
		out = append(out, japaneseAnnotationSpan{
			TokenIndex: idx,
			Span:       span,
		})
		searchFrom = end + 1
	}
	return out
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

func normalizeJapaneseSpanRange(runes []rune, span dto.PodcastTokenSpan) (dto.PodcastTokenSpan, bool) {
	firstHan := -1
	lastHan := -1
	for i := span.StartIndex; i <= span.EndIndex; i++ {
		if i < 0 || i >= len(runes) {
			return dto.PodcastTokenSpan{}, false
		}
		if dto.ContainsJapaneseKanji(string(runes[i])) {
			if firstHan == -1 {
				firstHan = i
			}
			lastHan = i
		}
	}
	if firstHan == -1 {
		return dto.PodcastTokenSpan{}, false
	}
	for firstHan > 0 && dto.ContainsJapaneseKanji(string(runes[firstHan-1])) {
		firstHan--
	}
	for lastHan+1 < len(runes) && dto.ContainsJapaneseKanji(string(runes[lastHan+1])) {
		lastHan++
	}
	span.StartIndex = firstHan
	span.EndIndex = lastHan
	return span, true
}

func japaneseDisplayText(seg dto.PodcastSegment) string {
	return strings.TrimSpace(seg.Text)
}

func japaneseTTSText(seg dto.PodcastSegment) string {
	return strings.TrimSpace(seg.Text)
}
