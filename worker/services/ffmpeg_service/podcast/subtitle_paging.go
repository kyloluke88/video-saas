package podcast

import (
	"strings"
	"unicode"

	"worker/internal/dto"
)

func subtitlePageCharLimit(layout subtitleLayout) int {
	return maxInt(1, layout.MaxLineChars)
}

type subtitlePageWindow struct {
	StartMS int
	EndMS   int
}

// Subtitle paging is display-only: we keep the original segment/audio intact,
// and only split a long segment into multiple on-screen pages.
func buildSubtitlePageWindows(segmentStartMS, segmentEndMS int, rawPageStarts []int, pageWeights []int) []subtitlePageWindow {
	pageCount := len(rawPageStarts)
	if pageCount == 0 {
		pageCount = len(pageWeights)
	}
	if pageCount == 0 {
		return nil
	}
	if segmentEndMS <= segmentStartMS {
		segmentEndMS = segmentStartMS + pageCount
	}
	if len(rawPageStarts) != pageCount {
		rawPageStarts = make([]int, pageCount)
	}
	if len(pageWeights) != pageCount {
		pageWeights = make([]int, pageCount)
		for i := range pageWeights {
			pageWeights[i] = 1
		}
	}

	starts := make([]int, pageCount)
	starts[0] = segmentStartMS
	duration := maxInt(1, segmentEndMS-segmentStartMS)
	totalWeight := 0
	for _, weight := range pageWeights {
		totalWeight += maxInt(1, weight)
	}
	accWeight := 0

	for i := 1; i < len(rawPageStarts); i++ {
		accWeight += maxInt(1, pageWeights[i-1])
		fallback := segmentStartMS + (duration*accWeight)/maxInt(1, totalWeight)
		candidate := rawPageStarts[i]
		if candidate <= starts[i-1] || candidate >= segmentEndMS {
			candidate = fallback
		}
		if candidate <= starts[i-1] {
			candidate = starts[i-1] + 1
		}
		if candidate >= segmentEndMS {
			candidate = segmentEndMS - 1
		}
		starts[i] = candidate
	}

	out := make([]subtitlePageWindow, 0, len(starts))
	for i, start := range starts {
		end := segmentEndMS
		if i+1 < len(starts) {
			end = starts[i+1] - 10
		}
		if end <= start {
			end = start + 1
		}
		out = append(out, subtitlePageWindow{
			StartMS: start,
			EndMS:   end,
		})
	}
	return out
}

func subtitleRuneCount(text string) int {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return 0
	}
	return len([]rune(trimmed))
}

func subtitleEndsWithPunctuation(text string) bool {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return false
	}
	runes := []rune(trimmed)
	return isPunctuationRune(runes[len(runes)-1])
}

func adjustSubtitlePageBreak(texts []string, start, end int) int {
	if end <= start {
		return end
	}
	for {
		end = extendSubtitleBreakForAttachedSuffixes(texts, end)
		if end >= len(texts) {
			return end
		}
		stack := subtitleWrapperStack(texts, start, end)
		if len(stack) == 0 {
			return end
		}
		advanced := false
		for end < len(texts) {
			stack = subtitleApplyWrapperText(stack, texts[end])
			end++
			advanced = true
			if len(stack) == 0 {
				break
			}
		}
		if !advanced {
			return end
		}
	}
}

func extendSubtitleBreakForAttachedSuffixes(texts []string, end int) int {
	for end < len(texts) && subtitleTextAttachesToPrevious(texts[end]) {
		end++
	}
	return end
}

func subtitleTextAttachesToPrevious(text string) bool {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return false
	}
	visible := false
	for _, r := range []rune(trimmed) {
		if unicode.IsSpace(r) {
			continue
		}
		visible = true
		if !isSubtitleTrailingAttachedRune(r) {
			return false
		}
	}
	return visible
}

func subtitleWrapperStack(texts []string, start, end int) []rune {
	stack := make([]rune, 0, 4)
	for i := start; i < end && i < len(texts); i++ {
		stack = subtitleApplyWrapperText(stack, texts[i])
	}
	return stack
}

func subtitleApplyWrapperText(stack []rune, text string) []rune {
	for _, r := range []rune(strings.TrimSpace(text)) {
		if closing, ok := subtitleClosingWrapperFor(r); ok {
			stack = append(stack, closing)
			continue
		}
		if len(stack) > 0 && r == stack[len(stack)-1] {
			stack = stack[:len(stack)-1]
		}
	}
	return stack
}

func subtitleClosingWrapperFor(r rune) (rune, bool) {
	switch r {
	case '（':
		return '）', true
	case '(':
		return ')', true
	case '【':
		return '】', true
	case '[':
		return ']', true
	case '《':
		return '》', true
	case '〈':
		return '〉', true
	case '「':
		return '」', true
	case '『':
		return '』', true
	case '“':
		return '”', true
	case '‘':
		return '’', true
	default:
		return 0, false
	}
}

func isSubtitleTrailingAttachedRune(r rune) bool {
	return isTrailingAttachedPunctuationRune(r) || isSubtitleClosingWrapperRune(r)
}

func isTrailingAttachedPunctuationRune(r rune) bool {
	return strings.ContainsRune("，。！？；：、…,.!?;:，。、！？；：", r)
}

func isSubtitleClosingWrapperRune(r rune) bool {
	switch r {
	case '）', ')', '】', ']', '》', '〉', '」', '』', '”', '’':
		return true
	default:
		return false
	}
}

func inlineLatinWordTokenRun(tokens []dto.PodcastToken, start int) (int, bool) {
	if start < 0 || start >= len(tokens) || !isLatinWordBodyToken(tokens[start].Char) {
		return 0, false
	}
	end := start
	for end+1 < len(tokens) {
		switch {
		case isLatinWordBodyToken(tokens[end+1].Char):
			end++
		case isLatinWordConnectorToken(tokens[end+1].Char) && end+2 < len(tokens) && isLatinWordBodyToken(tokens[end+2].Char):
			end++
		default:
			return end, true
		}
	}
	return end, true
}

func isLatinWordBodyToken(text string) bool {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return false
	}
	for _, r := range []rune(trimmed) {
		if unicode.In(unicode.ToLower(r), unicode.Latin) || unicode.IsDigit(r) {
			continue
		}
		return false
	}
	return true
}

func isLatinWordConnectorToken(text string) bool {
	trimmed := strings.TrimSpace(text)
	rs := []rune(trimmed)
	return len(rs) == 1 && (rs[0] == '-' || rs[0] == '\'')
}

func clampWindow(startMS, endMS, windowStartMS, windowEndMS int) (int, int, bool) {
	start := maxInt(startMS, windowStartMS)
	end := minInt(endMS, windowEndMS)
	if end <= start {
		return 0, 0, false
	}
	return start, end, true
}
