package podcast_audio_service

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"worker/internal/dto"
	ffmpegcommon "worker/services/ffmpeg_service/common"
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
	return state, true, nil
}

func persistBlockCheckpoint(dir string, index int, block dto.PodcastBlock, durationMS int) error {
	state := blockCheckpoint{
		Block:      block,
		DurationMS: durationMS,
	}
	return writeJSON(blockStatePath(dir, index, block.BlockID), state)
}

func blockCheckpointComplete(language string, state blockCheckpoint, audioPath string) bool {
	if !fileExists(audioPath) || len(state.Block.Segments) == 0 || state.DurationMS <= 0 {
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
