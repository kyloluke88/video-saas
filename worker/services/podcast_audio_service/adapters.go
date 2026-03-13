package podcast_audio_service

import (
	"strings"

	"worker/internal/dto"
	"worker/pkg/tts"
)

type scriptAdapter interface {
	SegmentText(seg dto.PodcastSegment) string
	NormalizeSegment(seg dto.PodcastSegment) dto.PodcastSegment
	ApplyAlignment(seg dto.PodcastSegment, subtitles []tts.Subtitle) dto.PodcastSegment
	AlignmentStats(seg dto.PodcastSegment) (int, int)
	CharacterCount(seg dto.PodcastSegment) int
}

func adapterFor(language string) scriptAdapter {
	switch strings.ToLower(strings.TrimSpace(language)) {
	case "ja", "ja-jp":
		return japaneseAdapter{}
	default:
		return chineseAdapter{}
	}
}

type japaneseAdapter struct{}

func (japaneseAdapter) SegmentText(seg dto.PodcastSegment) string {
	return japaneseTTSText(seg)
}

func (japaneseAdapter) NormalizeSegment(seg dto.PodcastSegment) dto.PodcastSegment {
	return normalizeJapaneseSegment(seg)
}

func (japaneseAdapter) ApplyAlignment(seg dto.PodcastSegment, subtitles []tts.Subtitle) dto.PodcastSegment {
	return seg
}

func (japaneseAdapter) AlignmentStats(seg dto.PodcastSegment) (int, int) {
	return japaneseAlignmentStats(seg)
}

func (japaneseAdapter) CharacterCount(seg dto.PodcastSegment) int {
	return japaneseCharacterCount(seg)
}
