package podcast_audio_service

import (
	"fmt"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"worker/internal/dto"
)

func buildJapaneseTokens(seg dto.PodcastSegment) []dto.PodcastToken {
	text := japaneseDisplayText(seg)
	runes := []rune(text)
	if len(runes) == 0 {
		return nil
	}

	out := make([]dto.PodcastToken, 0, len(runes))
	for i, r := range runes {
		_ = i
		out = append(out, dto.PodcastToken{Text: string(r)})
	}
	return out
}

func assignJapaneseTokenTimes(seg dto.PodcastSegment) dto.PodcastSegment {
	tokens := buildJapaneseTokens(seg)
	if len(tokens) == 0 {
		seg.Tokens = nil
		return seg
	}
	duration := seg.EndMS - seg.StartMS
	if duration <= 0 {
		seg.Tokens = tokens
		return seg
	}
	nonSpace := make([]int, 0, len(tokens))
	for i, token := range tokens {
		if strings.TrimSpace(token.Text) != "" {
			nonSpace = append(nonSpace, i)
		}
	}
	if len(nonSpace) == 0 {
		for i := range tokens {
			tokens[i].StartMS = seg.StartMS
			tokens[i].EndMS = seg.EndMS
		}
		seg.Tokens = tokens
		return seg
	}
	step := float64(duration) / float64(len(nonSpace))
	lastStart := seg.StartMS
	for order, idx := range nonSpace {
		start := seg.StartMS + int(step*float64(order))
		end := seg.StartMS + int(step*float64(order+1))
		if end <= start {
			end = start + 1
		}
		tokens[idx].StartMS = start
		tokens[idx].EndMS = end
		lastStart = end
	}
	for i := range tokens {
		if strings.TrimSpace(tokens[i].Text) != "" {
			continue
		}
		tokens[i].StartMS = seg.StartMS
		tokens[i].EndMS = maxInt(seg.StartMS, lastStart)
	}
	seg.Tokens = tokens
	return seg
}

func normalizeJapaneseSegment(seg dto.PodcastSegment) dto.PodcastSegment {
	text := japaneseDisplayText(seg)
	annotationTokens := seg.Tokens
	seg.TokenSpans = buildTokenSpansFromTokens(text, annotationTokens)
	seg = assignJapaneseTokenTimes(seg)
	return seg
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

func buildTokenSpansFromTokens(text string, tokens []dto.PodcastToken) []dto.PodcastTokenSpan {
	if len(tokens) == 0 {
		return nil
	}
	runes := []rune(text)
	if len(runes) == 0 {
		return nil
	}

	out := make([]dto.PodcastTokenSpan, 0, len(tokens))
	searchFrom := 0
	for _, token := range tokens {
		surface := strings.TrimSpace(token.Text)
		reading := strings.TrimSpace(token.Reading)
		if surface == "" || reading == "" {
			continue
		}
		matchStart, matchEnd, ok := findRubySurfaceRange(runes, []rune(surface), searchFrom)
		if !ok {
			continue
		}
		span, ok := normalizeTokenSpanRange(runes, dto.PodcastTokenSpan{
			StartIndex: matchStart,
			EndIndex:   matchEnd,
			Reading:    reading,
		})
		if !ok {
			searchFrom = matchEnd + 1
			continue
		}
		out = append(out, span)
		searchFrom = matchEnd + 1
	}

	return dedupeTokenSpans(out)
}

func dedupeTokenSpans(spans []dto.PodcastTokenSpan) []dto.PodcastTokenSpan {
	if len(spans) == 0 {
		return nil
	}
	sort.Slice(spans, func(i, j int) bool {
		if spans[i].StartIndex == spans[j].StartIndex {
			return spans[i].EndIndex < spans[j].EndIndex
		}
		return spans[i].StartIndex < spans[j].StartIndex
	})
	filtered := spans[:0]
	lastEnd := -1
	for _, span := range spans {
		if span.StartIndex <= lastEnd {
			continue
		}
		filtered = append(filtered, span)
		lastEnd = span.EndIndex
	}
	return filtered
}

func findRubySurfaceRange(textRunes, surfaceRunes []rune, searchFrom int) (int, int, bool) {
	if len(surfaceRunes) == 0 || len(textRunes) == 0 || searchFrom >= len(textRunes) {
		return 0, 0, false
	}
	maxStart := len(textRunes) - len(surfaceRunes)
	for start := maxInt(0, searchFrom); start <= maxStart; start++ {
		if runesEqual(textRunes[start:start+len(surfaceRunes)], surfaceRunes) {
			return start, start + len(surfaceRunes) - 1, true
		}
	}
	return 0, 0, false
}

func runesEqual(a, b []rune) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func validateJapaneseScriptInput(script dto.PodcastScript) error {
	if len(script.Blocks) == 0 {
		return fmt.Errorf("japanese podcast script requires non-empty blocks")
	}
	for _, block := range script.Blocks {
		if strings.TrimSpace(block.TTSBlockID) == "" {
			return fmt.Errorf("japanese podcast block requires tts_block_id")
		}
		if len(block.Segments) == 0 {
			return fmt.Errorf("japanese podcast block %s has no segments", block.TTSBlockID)
		}
		for _, seg := range block.Segments {
			if len(seg.TokenSpans) > 0 {
				return fmt.Errorf("segment %s uses deprecated token_spans in input; use tokens instead", seg.SegmentID)
			}
			if strings.TrimSpace(seg.SegmentID) == "" {
				return fmt.Errorf("japanese podcast segment_id is required")
			}
			if strings.TrimSpace(japaneseDisplayText(seg)) == "" {
				return fmt.Errorf("segment %s text is required", seg.SegmentID)
			}
		}
	}
	return nil
}

func normalizeTokenSpanRange(runes []rune, span dto.PodcastTokenSpan) (dto.PodcastTokenSpan, bool) {
	start := span.StartIndex
	end := span.EndIndex

	firstHan := -1
	lastHan := -1
	for i := start; i <= end; i++ {
		if unicode.In(runes[i], unicode.Han) {
			if firstHan == -1 {
				firstHan = i
			}
			lastHan = i
		}
	}
	if firstHan == -1 {
		return dto.PodcastTokenSpan{}, false
	}

	for firstHan > 0 && unicode.In(runes[firstHan-1], unicode.Han) {
		firstHan--
	}
	for lastHan+1 < len(runes) && unicode.In(runes[lastHan+1], unicode.Han) {
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
