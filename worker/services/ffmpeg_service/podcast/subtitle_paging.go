package podcast

import "strings"

const subtitlePageMaxChars = 20

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

func clampWindow(startMS, endMS, windowStartMS, windowEndMS int) (int, int, bool) {
	start := maxInt(startMS, windowStartMS)
	end := minInt(endMS, windowEndMS)
	if end <= start {
		return 0, 0, false
	}
	return start, end, true
}
