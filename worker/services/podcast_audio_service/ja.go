package podcast_audio_service

import (
	"fmt"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"worker/internal/dto"
)

func buildJapaneseChars(seg dto.PodcastSegment) []dto.PodcastCharToken {
	text := japaneseDisplayText(seg)
	runes := []rune(text)
	if len(runes) == 0 {
		return nil
	}

	out := make([]dto.PodcastCharToken, 0, len(runes))
	for i, r := range runes {
		out = append(out, dto.PodcastCharToken{
			Index: i,
			Char:  string(r),
		})
	}
	return out
}

func assignJapaneseCharTimes(seg dto.PodcastSegment) dto.PodcastSegment {
	chars := buildJapaneseChars(seg)
	if len(chars) == 0 {
		seg.Chars = nil
		return seg
	}
	duration := seg.EndMS - seg.StartMS
	if duration <= 0 {
		seg.Chars = chars
		return seg
	}
	nonSpace := make([]int, 0, len(chars))
	for i, ch := range chars {
		if strings.TrimSpace(ch.Char) != "" {
			nonSpace = append(nonSpace, i)
		}
	}
	if len(nonSpace) == 0 {
		for i := range chars {
			chars[i].StartMS = seg.StartMS
			chars[i].EndMS = seg.EndMS
		}
		seg.Chars = chars
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
		chars[idx].StartMS = start
		chars[idx].EndMS = end
		lastStart = end
	}
	for i := range chars {
		if strings.TrimSpace(chars[i].Char) != "" {
			continue
		}
		chars[i].StartMS = seg.StartMS
		chars[i].EndMS = maxInt(seg.StartMS, lastStart)
	}
	seg.Chars = chars
	return seg
}

func normalizeJapaneseSegment(seg dto.PodcastSegment) dto.PodcastSegment {
	text := japaneseDisplayText(seg)
	seg.RubySpans = buildRubySpansFromTokens(text, seg.RubyTokens)
	seg.RubyTokens = nil
	seg = assignJapaneseCharTimes(seg)
	return seg
}

func japaneseAlignmentStats(seg dto.PodcastSegment) (int, int) {
	matched := 0
	for _, ch := range seg.Chars {
		if ch.EndMS > ch.StartMS {
			matched++
		}
	}
	return matched, len(seg.Chars)
}

func japaneseCharacterCount(seg dto.PodcastSegment) int {
	return utf8.RuneCountInString(strings.TrimSpace(japaneseDisplayText(seg)))
}

func buildRubySpansFromTokens(text string, tokens []dto.PodcastRubyToken) []dto.PodcastRubySpan {
	if len(tokens) == 0 {
		return nil
	}
	runes := []rune(text)
	if len(runes) == 0 {
		return nil
	}

	out := make([]dto.PodcastRubySpan, 0, len(tokens))
	searchFrom := 0
	for _, token := range tokens {
		surface := strings.TrimSpace(token.Surface)
		reading := strings.TrimSpace(token.Reading)
		if surface == "" || reading == "" {
			continue
		}
		matchStart, matchEnd, ok := findRubySurfaceRange(runes, []rune(surface), searchFrom)
		if !ok {
			continue
		}
		span, ok := normalizeRubySpanRange(runes, dto.PodcastRubySpan{
			StartIndex: matchStart,
			EndIndex:   matchEnd,
			Ruby:       reading,
		})
		if !ok {
			searchFrom = matchEnd + 1
			continue
		}
		out = append(out, span)
		searchFrom = matchEnd + 1
	}

	return dedupeRubySpans(out)
}

func dedupeRubySpans(spans []dto.PodcastRubySpan) []dto.PodcastRubySpan {
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
			if len(seg.RubySpans) > 0 {
				return fmt.Errorf("segment %s uses deprecated ruby_spans; use ruby_tokens instead", seg.SegmentID)
			}
			if strings.TrimSpace(japaneseDisplayText(seg)) == "" {
				return fmt.Errorf("segment %s display_ja is required", seg.SegmentID)
			}
			if strings.TrimSpace(japaneseTTSText(seg)) == "" {
				return fmt.Errorf("segment %s tts_ja is required", seg.SegmentID)
			}
		}
	}
	return nil
}

func normalizeRubySpanRange(runes []rune, span dto.PodcastRubySpan) (dto.PodcastRubySpan, bool) {
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
		return dto.PodcastRubySpan{}, false
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
	if text := strings.TrimSpace(seg.DisplayJA); text != "" {
		return text
	}
	return strings.TrimSpace(seg.JA)
}

func japaneseTTSText(seg dto.PodcastSegment) string {
	if text := strings.TrimSpace(seg.TTSJA); text != "" {
		return text
	}
	if text := strings.TrimSpace(seg.DisplayJA); text != "" {
		return text
	}
	return strings.TrimSpace(seg.JA)
}
