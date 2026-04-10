package podcast_audio_service

import (
	"fmt"
	"strings"
	"unicode/utf8"

	dto "worker/services/podcast/model"
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
	refs := dto.BuildJapaneseTokenSpanRefs(japaneseDisplayText(seg), seg.Tokens)
	if len(refs) == 0 {
		return nil
	}
	out := make([]japaneseAnnotationSpan, 0, len(refs))
	for _, ref := range refs {
		out = append(out, japaneseAnnotationSpan{
			TokenIndex: ref.TokenIndex,
			Span:       ref.Span,
		})
	}
	return out
}

func japaneseDisplayText(seg dto.PodcastSegment) string {
	return strings.TrimSpace(seg.Text)
}

func japaneseTTSText(seg dto.PodcastSegment) string {
	return strings.TrimSpace(seg.Text)
}
