package podcast

import "strings"

const subtitlePageMaxChars = 20

type subtitlePageWindow struct {
	StartMS int
	EndMS   int
}

// Subtitle paging is display-only: we keep the original segment/audio intact,
// and only split a long segment into multiple on-screen pages.
func buildSubtitlePageWindows(segmentStartMS, segmentEndMS int, rawPageStarts []int) []subtitlePageWindow {
	if len(rawPageStarts) == 0 {
		return nil
	}
	if segmentEndMS <= segmentStartMS {
		segmentEndMS = segmentStartMS + len(rawPageStarts)
	}

	starts := make([]int, len(rawPageStarts))
	starts[0] = segmentStartMS
	duration := maxInt(1, segmentEndMS-segmentStartMS)

	for i := 1; i < len(rawPageStarts); i++ {
		candidate := rawPageStarts[i]
		fallback := segmentStartMS + (duration*i)/len(rawPageStarts)
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
			end = starts[i+1]
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
