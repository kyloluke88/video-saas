package podcast_audio_service

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	services "worker/services"
	dto "worker/services/podcast/model"
)

type audioArtifacts struct {
	projectDir     string
	dialoguePath   string
	alignedPath    string
	rawBlocksDir   string
	blocksDir      string
	segmentsDir    string
	blockStatesDir string
	ttsDebugDir    string
	blockGapPath   string
}

func prepareAudioArtifacts(projectDir string) (audioArtifacts, error) {
	artifacts := audioArtifacts{
		projectDir:     projectDir,
		dialoguePath:   filepath.Join(projectDir, "dialogue.mp3"),
		alignedPath:    projectScriptAlignedPath(projectDir),
		rawBlocksDir:   filepath.Join(projectDir, "raw_blocks"),
		blocksDir:      filepath.Join(projectDir, "blocks"),
		segmentsDir:    filepath.Join(projectDir, "segments"),
		blockStatesDir: filepath.Join(projectDir, "block_states"),
		ttsDebugDir:    filepath.Join(projectDir, "tts_debug"),
		blockGapPath:   filepath.Join(projectDir, "block_gap.wav"),
	}
	for _, dir := range []string{
		artifacts.rawBlocksDir,
		artifacts.blocksDir,
		artifacts.segmentsDir,
		artifacts.blockStatesDir,
		artifacts.ttsDebugDir,
		chunkWorkingDir(projectDir),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return audioArtifacts{}, err
		}
	}
	return artifacts, nil
}

func finalizeAlignedScript(projectID, alignedPath, dialoguePath string, script dto.PodcastScript) (dto.PodcastScript, error) {
	finalScript := script
	if err := validateAlignedTimeline(finalScript, dialoguePath); err != nil {
		return dto.PodcastScript{}, err
	}

	if err := writeJSON(alignedPath, finalScript); err != nil {
		return dto.PodcastScript{}, err
	}
	return finalScript, nil
}

func validateAlignedTimeline(script dto.PodcastScript, dialoguePath string) error {
	segments := script.FlatSegments()
	if len(segments) == 0 {
		return nil
	}

	durationMS := 0
	if fileExists(dialoguePath) {
		value, err := audioDurationMS(dialoguePath)
		if err != nil {
			return err
		}
		durationMS = value
	}

	prevEnd := 0
	for i, seg := range segments {
		startMS := seg.StartMS
		endMS := seg.EndMS
		if endMS <= startMS {
			return markAlignedTimelineNonRetryable(fmt.Errorf("invalid aligned timeline at segment %s: start_ms=%d end_ms=%d", seg.SegmentID, startMS, endMS))
		}
		if i > 0 && startMS < prevEnd {
			return markAlignedTimelineNonRetryable(fmt.Errorf("non-monotonic aligned timeline at segment %s: start_ms=%d prev_end_ms=%d", seg.SegmentID, startMS, prevEnd))
		}
		prevEnd = endMS
	}

	if durationMS > 0 && prevEnd > durationMS+1000 {
		return markAlignedTimelineNonRetryable(fmt.Errorf("aligned timeline exceeds dialogue audio: last_end_ms=%d audio_duration_ms=%d", prevEnd, durationMS))
	}
	return nil
}

func markAlignedTimelineNonRetryable(err error) error {
	if err == nil {
		return nil
	}
	var nonRetryable services.NonRetryableError
	if errors.As(err, &nonRetryable) {
		return err
	}
	return services.NonRetryableError{Err: err}
}

func cleanupGoogleTTSDebugArtifacts(projectDir string) error {
	if strings.TrimSpace(projectDir) == "" {
		return nil
	}

	patterns := []string{
		filepath.Join(projectDir, "tts_debug", "*.google_request.json"),
		filepath.Join(projectDir, "block_states", "*.google_request.json"),
		filepath.Join(projectDir, "block_states", "*.pre_tempo.*"),
	}

	var errs []error
	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			errs = append(errs, fmt.Errorf("glob %s: %w", pattern, err))
			continue
		}
		for _, path := range matches {
			if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
				errs = append(errs, fmt.Errorf("remove %s: %w", path, err))
			}
		}
	}

	return errors.Join(errs...)
}
