package podcast_audio_service

import (
	"fmt"
	"strings"
	"unicode"

	"worker/internal/dto"
)

func validateChineseScriptInput(script dto.PodcastScript) error {
	if len(script.Blocks) == 0 {
		return fmt.Errorf("chinese podcast script requires non-empty blocks")
	}
	for _, block := range script.Blocks {
		if strings.TrimSpace(block.BlockID) == "" {
			return fmt.Errorf("chinese podcast block requires block_id")
		}
		if len(block.Segments) == 0 {
			return fmt.Errorf("chinese podcast block %s has no segments", block.BlockID)
		}
		for _, seg := range block.Segments {
			if strings.TrimSpace(seg.SegmentID) == "" {
				return fmt.Errorf("chinese podcast segment_id is required")
			}
			if strings.TrimSpace(seg.Text) == "" {
				return fmt.Errorf("segment %s text is required", seg.SegmentID)
			}
			if len(seg.Tokens) == 0 {
				return fmt.Errorf("segment %s tokens are required", seg.SegmentID)
			}
			expectedCount := chineseVisibleRuneCount(seg.Text)
			actualCount := chineseVisibleTokenRuneCount(seg.Tokens)
			if actualCount != expectedCount {
				return fmt.Errorf("segment %s token coverage mismatch expected=%d actual=%d", seg.SegmentID, expectedCount, actualCount)
			}
			for _, token := range seg.Tokens {
				if chineseWhitespaceToken(token) {
					continue
				}
				if strings.TrimSpace(token.Char) == "" {
					return fmt.Errorf("segment %s token char is required", seg.SegmentID)
				}
			}
		}
	}
	return nil
}

func chineseVisibleRuneCount(text string) int {
	count := 0
	for _, r := range []rune(strings.TrimSpace(text)) {
		if unicode.IsSpace(r) {
			continue
		}
		count++
	}
	return count
}

func chineseVisibleTokenRuneCount(tokens []dto.PodcastToken) int {
	count := 0
	for _, token := range tokens {
		for _, r := range []rune(token.Char) {
			if unicode.IsSpace(r) {
				continue
			}
			count++
		}
	}
	return count
}

func chineseWhitespaceToken(token dto.PodcastToken) bool {
	return strings.TrimSpace(token.Char) == "" && token.Char != ""
}

func chineseSegmentTokens(seg dto.PodcastSegment) []dto.PodcastToken {
	if len(seg.Tokens) == 0 {
		return nil
	}
	out := make([]dto.PodcastToken, len(seg.Tokens))
	copy(out, seg.Tokens)
	return out
}

// fillUnalignedChineseTokens distributes any still-missing token timing across the
// segment window. The aligner prefers real MFA word timings, but this fallback
// keeps the renderer usable when alignment is partial or noisy.
func fillUnalignedChineseTokens(tokens []dto.PodcastToken, segmentStartMS, segmentEndMS int) {
	if len(tokens) == 0 {
		return
	}

	start := maxInt(0, segmentStartMS)
	end := maxInt(start+1, segmentEndMS)

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
}
