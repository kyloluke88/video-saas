package podcast_audio_service

import (
	"fmt"
	"log"
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
	IsMultiple *int             `json:"is_multiple,omitempty"`
}

type blockCheckpointIndex struct {
	Version int                                  `json:"version"`
	Blocks  map[string]blockCheckpointIndexEntry `json:"blocks"`
}

type blockCheckpointIndexEntry struct {
	State         blockCheckpoint `json:"state"`
	FileSize      int64           `json:"file_size"`
	FileModUnixNS int64           `json:"file_mod_unix_ns"`
}

type blockCheckpointStore struct {
	dir       string
	indexPath string
	index     blockCheckpointIndex
	loaded    bool
	dirty     bool
}

const blockCheckpointIndexVersion = 1

func blockStatePath(dir string, index int, blockID string) string {
	return unitAudioPath(dir, index, blockID, "json")
}

func newBlockCheckpointStore(dir string) *blockCheckpointStore {
	return &blockCheckpointStore{
		dir:       dir,
		indexPath: filepath.Join(dir, "index.json"),
	}
}

func (s *blockCheckpointStore) loadBlockCheckpoint(index int, blockID string) (blockCheckpoint, bool, error) {
	if s == nil {
		return blockCheckpoint{}, false, nil
	}

	path := blockStatePath(s.dir, index, blockID)
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return blockCheckpoint{}, false, nil
		}
		return blockCheckpoint{}, false, err
	}
	if err := s.ensureLoaded(); err != nil {
		return blockCheckpoint{}, false, err
	}

	key := blockCheckpointKey(index, blockID)
	if entry, ok := s.index.Blocks[key]; ok && entry.matches(info) {
		if err := validateBlockStateSegments(path, entry.State.Block); err != nil {
			return blockCheckpoint{}, false, err
		}
		return entry.State, true, nil
	}

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
	s.index.Blocks[key] = newBlockCheckpointIndexEntry(state, info)
	s.dirty = true
	return state, true, nil
}

func (s *blockCheckpointStore) persistBlockCheckpoint(index int, block dto.PodcastBlock, durationMS int, isMultiple *int) error {
	if s == nil {
		return nil
	}

	path := blockStatePath(s.dir, index, block.BlockID)
	if err := validateBlockStateSegments(path, block); err != nil {
		return err
	}
	state := blockCheckpoint{
		Block:      block,
		DurationMS: durationMS,
		IsMultiple: isMultiple,
	}
	if err := writeJSON(path, state); err != nil {
		return err
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if err := s.ensureLoaded(); err != nil {
		return err
	}
	s.index.Blocks[blockCheckpointKey(index, block.BlockID)] = newBlockCheckpointIndexEntry(state, info)
	s.dirty = true
	return nil
}

func (s *blockCheckpointStore) flush() error {
	if s == nil || !s.loaded || !s.dirty {
		return nil
	}
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return err
	}
	s.index.Version = blockCheckpointIndexVersion
	if s.index.Blocks == nil {
		s.index.Blocks = make(map[string]blockCheckpointIndexEntry)
	}
	if err := writeJSON(s.indexPath, s.index); err != nil {
		return err
	}
	s.dirty = false
	return nil
}

func (s *blockCheckpointStore) ensureLoaded() error {
	if s == nil || s.loaded {
		return nil
	}
	s.loaded = true
	s.index = blockCheckpointIndex{
		Version: blockCheckpointIndexVersion,
		Blocks:  make(map[string]blockCheckpointIndexEntry),
	}

	var index blockCheckpointIndex
	if err := readJSON(s.indexPath, &index); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		log.Printf("⚠️ podcast block checkpoint index ignored dir=%s err=%v", s.dir, err)
		return nil
	}
	if index.Blocks == nil {
		index.Blocks = make(map[string]blockCheckpointIndexEntry)
	}
	index.Version = blockCheckpointIndexVersion
	s.index = index
	return nil
}

func (e blockCheckpointIndexEntry) matches(info os.FileInfo) bool {
	if info == nil {
		return false
	}
	return e.FileSize == info.Size() && e.FileModUnixNS == info.ModTime().UnixNano()
}

func newBlockCheckpointIndexEntry(state blockCheckpoint, info os.FileInfo) blockCheckpointIndexEntry {
	return blockCheckpointIndexEntry{
		State:         state,
		FileSize:      info.Size(),
		FileModUnixNS: info.ModTime().UnixNano(),
	}
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

func blockCheckpointComplete(language string, state blockCheckpoint, audioPath string) bool {
	if !blockCheckpointHasAudio(state, audioPath) {
		return false
	}
	return blockHasAlignedTiming(language, state.Block)
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

func blockCheckpointKey(index int, blockID string) string {
	return unitAudioFilename(index, blockID, "json")
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
