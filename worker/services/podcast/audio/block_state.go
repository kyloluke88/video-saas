package podcast_audio_service

import (
	"fmt"
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
}

func blockStatePath(dir string, index int, blockID string) string {
	return unitAudioPath(dir, index, blockID, "json")
}

func loadBlockCheckpoint(dir string, index int, blockID string) (blockCheckpoint, bool, error) {
	path := blockStatePath(dir, index, blockID)
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return blockCheckpoint{}, false, nil
		}
		return blockCheckpoint{}, false, err
	}
	var state blockCheckpoint
	if err := readJSON(path, &state); err != nil {
		return blockCheckpoint{}, false, err
	}
	if err := validateBlockStateSegments(path, state.Block); err != nil {
		return blockCheckpoint{}, false, err
	}
	return state, true, nil
}

func persistBlockCheckpoint(dir string, index int, block dto.PodcastBlock, durationMS int) error {
	path := blockStatePath(dir, index, block.BlockID)
	if err := validateBlockStateSegments(path, block); err != nil {
		return err
	}
	state := blockCheckpoint{
		Block:      block,
		DurationMS: durationMS,
	}
	return writeJSON(path, state)
}

func blockCheckpointComplete(language string, state blockCheckpoint, audioPath string) bool {
	if !blockCheckpointHasAudio(state, audioPath) {
		return false
	}
	for _, seg := range state.Block.Segments {
		if seg.EndMS <= seg.StartMS {
			return false
		}
		if len(seg.Tokens) == 0 {
			return false
		}
	}
	return true
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
	ext = strings.TrimPrefix(strings.TrimSpace(ext), ".")
	if ext == "" {
		ext = "mp3"
	}
	return filepath.Join(dir, fmt.Sprintf("%03d_%s.%s", index+1, sanitizeSegmentID(unitID), ext))
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
