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

func chineseSegmentTokens(seg dto.PodcastSegment) []dto.PodcastToken {
	if len(seg.Tokens) == 0 {
		return nil
	}
	out := make([]dto.PodcastToken, len(seg.Tokens))
	copy(out, seg.Tokens)
	return out
}

func fillUnalignedChineseTokens(tokens []dto.PodcastToken, segmentStartMS, segmentEndMS int) {
	lastEnd := segmentStartMS
	for i := range tokens {
		if tokens[i].StartMS > 0 || tokens[i].EndMS > 0 {
			lastEnd = maxInt(lastEnd, tokens[i].EndMS)
			continue
		}
		nextStart := segmentEndMS
		for j := i + 1; j < len(tokens); j++ {
			if tokens[j].StartMS > 0 || tokens[j].EndMS > 0 {
				nextStart = firstPositive(tokens[j].StartMS, tokens[j].EndMS, segmentEndMS)
				break
			}
		}
		tokens[i].StartMS = lastEnd
		tokens[i].EndMS = maxInt(lastEnd, nextStart)
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
