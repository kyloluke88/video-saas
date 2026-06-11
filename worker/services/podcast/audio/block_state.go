package podcast_audio_service

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"

	services "worker/services"
	ffmpegcommon "worker/services/media/ffmpeg/common"
	dto "worker/services/podcast/model"
)

type blockCheckpoint struct {
	Block      dto.PodcastBlock `json:"block"`
	DurationMS int              `json:"duration_ms"`
	Tempo      float64          `json:"tempo,omitempty"`
}

func blockStatePath(dir string, index int, blockID string) string {
	return unitAudioPath(dir, index, blockID, "json")
}

func loadBlockCheckpoint(dir string, index int, blockID string) (blockCheckpoint, bool, error) {
	path := blockStatePath(dir, index, blockID)
	var state blockCheckpoint
	if err := readJSON(path, &state); err != nil {
		if os.IsNotExist(err) {
			return blockCheckpoint{}, false, nil
		}
		return blockCheckpoint{}, false, err
	}
	if err := validateBlockStateSegments(path, state.Block); err != nil {
		return blockCheckpoint{}, false, err
	}
	return state, true, nil
}

func persistBlockCheckpoint(dir string, index int, block dto.PodcastBlock, durationMS int, tempo float64) error {
	path := blockStatePath(dir, index, block.BlockID)
	if err := validateBlockStateSegments(path, block); err != nil {
		return err
	}
	return writeJSON(path, blockCheckpoint{
		Block:      block,
		DurationMS: durationMS,
		Tempo:      tempo,
	})
}

func blockHasAlignedTiming(language string, block dto.PodcastBlock) bool {
	if len(block.Segments) == 0 {
		return false
	}

	prevEnd := 0
	for _, seg := range block.Segments {
		if seg.StartMS < 0 || seg.EndMS <= seg.StartMS {
			return false
		}
		if seg.StartMS < prevEnd {
			return false
		}
		if isJapaneseLanguage(language) {
			if !blockCheckpointJapaneseSegmentHasTiming(seg) {
				return false
			}
		} else if !blockCheckpointChineseSegmentHasTiming(seg) {
			return false
		}
		prevEnd = seg.EndMS
	}
	return true
}

func blockCheckpointComplete(language string, state blockCheckpoint, audioPath string, tempo float64, allowLegacyTempo bool) bool {
	if !blockCheckpointHasAudio(state, audioPath) {
		return false
	}
	if !blockCheckpointTempoMatches(state, tempo, allowLegacyTempo) {
		return false
	}
	return blockHasAlignedTiming(language, state.Block)
}

func blockCheckpointTempoMatches(state blockCheckpoint, tempo float64, allowLegacyTempo bool) bool {
	if tempo <= 0 {
		return math.Abs(state.Tempo) <= 0.001
	}
	if math.Abs(state.Tempo) <= 0.001 {
		return allowLegacyTempo
	}
	return math.Abs(state.Tempo-tempo) <= 0.001
}

func blockCheckpointChineseSegmentHasTiming(seg dto.PodcastSegment) bool {
	if len(seg.Tokens) == 0 {
		return false
	}
	for _, token := range seg.Tokens {
		if token.StartMS < 0 || token.EndMS <= token.StartMS {
			return false
		}
	}
	return true
}

func blockCheckpointJapaneseSegmentHasTiming(seg dto.PodcastSegment) bool {
	hasTokenTiming := false
	for _, token := range seg.Tokens {
		if token.StartMS < 0 || token.EndMS <= token.StartMS {
			return false
		}
		hasTokenTiming = true
	}

	hasHighlightTiming := false
	for _, span := range seg.HighlightSpans {
		if span.StartMS < 0 || span.EndMS <= span.StartMS {
			return false
		}
		hasHighlightTiming = true
	}

	return hasTokenTiming || hasHighlightTiming
}

func blockCheckpointHasAudio(state blockCheckpoint, audioPath string) bool {
	return fileExists(audioPath) && len(state.Block.Segments) > 0 && state.DurationMS > 0
}

func audioDurationMS(path string) (int, error) {
	sec, err := ffmpegcommon.AudioDurationSec(path)
	if err != nil {
		return 0, err
	}
	return int(sec * 1000), nil
}

func fileExists(path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

func unitAudioPath(dir string, index int, unitID, ext string) string {
	return filepath.Join(dir, unitAudioFilename(index, unitID, ext))
}

func unitAudioFilename(index int, unitID, ext string) string {
	ext = strings.TrimPrefix(strings.TrimSpace(ext), ".")
	if ext == "" {
		ext = "mp3"
	}
	return fmt.Sprintf("%03d_%s.%s", index+1, sanitizeSegmentID(unitID), ext)
}

func validateBlockStateSegments(path string, block dto.PodcastBlock) error {
	for _, seg := range block.Segments {
		if seg.StartMS == 0 && seg.EndMS == 0 {
			continue
		}
		durationMS := seg.EndMS - seg.StartMS
		if durationMS <= 1 {
			return services.NonRetryableError{
				Err: fmt.Errorf(
					"suspicious block_state segment timing block=%s segment=%s start_ms=%d end_ms=%d path=%s",
					strings.TrimSpace(block.BlockID),
					strings.TrimSpace(seg.SegmentID),
					seg.StartMS,
					seg.EndMS,
					strings.TrimSpace(path),
				),
			}
		}
	}
	return nil
}
