package podcast_audio_service

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"worker/internal/dto"
	"worker/pkg/tts"
)

type chineseAdapter struct{}

func (chineseAdapter) SegmentText(seg dto.PodcastSegment) string {
	return strings.TrimSpace(seg.ZH)
}

func (chineseAdapter) NormalizeSegment(seg dto.PodcastSegment) dto.PodcastSegment {
	seg.Tokens = chineseSegmentTokens(seg)
	return seg
}

func (chineseAdapter) ApplyAlignment(seg dto.PodcastSegment, subtitles []tts.Subtitle) dto.PodcastSegment {
	seg.Tokens = alignChineseTokens(seg, subtitles)
	return seg
}

func (chineseAdapter) AlignmentStats(seg dto.PodcastSegment) (int, int) {
	return chineseAlignmentStats(seg)
}

func (chineseAdapter) CharacterCount(seg dto.PodcastSegment) int {
	return chineseCharacterCount(seg)
}

func validateChineseScriptInput(script dto.PodcastScript) error {
	if len(script.Blocks) == 0 {
		return fmt.Errorf("chinese podcast script requires non-empty blocks")
	}
	for _, block := range script.Blocks {
		if strings.TrimSpace(block.TTSBlockID) == "" {
			return fmt.Errorf("chinese podcast block requires tts_block_id")
		}
		if len(block.Segments) == 0 {
			return fmt.Errorf("chinese podcast block %s has no segments", block.TTSBlockID)
		}
		for _, seg := range block.Segments {
			if strings.TrimSpace(seg.SegmentID) == "" {
				return fmt.Errorf("chinese podcast segment_id is required")
			}
			if strings.TrimSpace(seg.ZH) == "" {
				return fmt.Errorf("segment %s zh is required", seg.SegmentID)
			}
			if len(seg.Tokens) == 0 {
				return fmt.Errorf("segment %s tokens are required", seg.SegmentID)
			}
			for _, tk := range seg.Tokens {
				if strings.TrimSpace(tk.Char) == "" {
					return fmt.Errorf("segment %s token char is required", seg.SegmentID)
				}
			}
		}
	}
	return nil
}

func alignChineseTokens(seg dto.PodcastSegment, subtitles []tts.Subtitle) []dto.PodcastToken {
	tokens := chineseSegmentTokens(seg)
	if len(tokens) == 0 {
		return nil
	}

	if alignChineseTokensByRange(tokens, subtitles, seg.StartMS) {
		fillUnalignedChineseTokens(tokens, seg.StartMS, seg.EndMS)
		return tokens
	}

	subIdx := 0
	for i := range tokens {
		charText := strings.TrimSpace(tokens[i].Char)
		if charText == "" || isSilentToken(charText) {
			continue
		}
		for subIdx < len(subtitles) {
			sub := subtitles[subIdx]
			if strings.TrimSpace(sub.Text) == "" {
				subIdx++
				continue
			}
			if subtitleMatchesToken(sub.Text, tokens[i].Char) {
				tokens[i].StartMS = seg.StartMS + maxInt(0, sub.BeginTime)
				tokens[i].EndMS = seg.StartMS + maxInt(sub.BeginTime, sub.EndTime)
				subIdx++
				break
			}
			subIdx++
		}
	}

	fillUnalignedChineseTokens(tokens, seg.StartMS, seg.EndMS)
	return tokens
}

func alignChineseTokensByRange(tokens []dto.PodcastToken, subtitles []tts.Subtitle, segmentStartMS int) bool {
	matched := 0
	for _, sub := range subtitles {
		startIdx, endIdx, ok := chineseSubtitleTokenRange(tokens, sub)
		if !ok {
			continue
		}

		covered := make([]int, 0, endIdx-startIdx+1)
		for i := startIdx; i <= endIdx; i++ {
			charText := strings.TrimSpace(tokens[i].Char)
			if charText == "" || isSilentToken(charText) {
				continue
			}
			covered = append(covered, i)
		}
		if len(covered) == 0 {
			continue
		}

		beginMS := segmentStartMS + maxInt(0, sub.BeginTime)
		endMS := segmentStartMS + maxInt(sub.BeginTime, sub.EndTime)
		if endMS <= beginMS {
			endMS = beginMS + len(covered)
		}

		duration := endMS - beginMS
		for order, tokenIdx := range covered {
			tokenStart := beginMS + duration*order/len(covered)
			tokenEnd := beginMS + duration*(order+1)/len(covered)
			if tokenEnd <= tokenStart {
				tokenEnd = tokenStart + 1
			}
			tokens[tokenIdx].StartMS = tokenStart
			tokens[tokenIdx].EndMS = tokenEnd
			matched++
		}
	}
	return matched > 0
}

func chineseSubtitleTokenRange(tokens []dto.PodcastToken, sub tts.Subtitle) (int, int, bool) {
	if len(tokens) == 0 {
		return 0, 0, false
	}

	start := sub.BeginIndex
	end := sub.EndIndex
	if start < 0 || end < start {
		return 0, 0, false
	}

	textRunes := []rune(strings.TrimSpace(sub.Text))
	if len(textRunes) == 0 {
		return 0, 0, false
	}

	// Tencent's EndIndex is occasionally inclusive and occasionally behaves like
	// an exclusive bound depending on the subtitle payload; normalize against the
	// returned text length so we still get stable per-char timing.
	if end-start == len(textRunes) {
		end--
	}
	if end-start+1 < len(textRunes) {
		end = start + len(textRunes) - 1
	}
	if start >= len(tokens) {
		return 0, 0, false
	}
	if end >= len(tokens) {
		end = len(tokens) - 1
	}
	if end < start {
		return 0, 0, false
	}
	return start, end, true
}

func chineseSegmentTokens(seg dto.PodcastSegment) []dto.PodcastToken {
	if len(seg.Tokens) == 0 {
		return nil
	}
	out := make([]dto.PodcastToken, len(seg.Tokens))
	copy(out, seg.Tokens)
	return out
}

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
			windowEnd = windowStart + (j - i)
		}

		step := maxInt(1, (windowEnd-windowStart)/maxInt(1, j-i))
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

func chineseAlignmentStats(seg dto.PodcastSegment) (int, int) {
	matched := 0
	for _, token := range seg.Tokens {
		if token.EndMS > token.StartMS {
			matched++
		}
	}
	return matched, len(seg.Tokens)
}

func chineseCharacterCount(seg dto.PodcastSegment) int {
	return utf8.RuneCountInString(strings.TrimSpace(seg.ZH))
}
