package pipeline

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"worker/internal/workspace"
)

func InvalidateOutputs(projectID string, ttsType int, startFrom string) error {
	stage, ok := ParseStage(startFrom)
	if !ok {
		return fmt.Errorf("unsupported podcast stage: %s", strings.TrimSpace(startFrom))
	}

	projectDir := workspace.ProjectDir(projectID)
	var errs []error

	switch stage {
	case StageGenerate:
		errs = append(errs, removeFiles(
			filepath.Join(projectDir, "dialogue.mp3"),
			filepath.Join(projectDir, "script_aligned.json"),
			filepath.Join(projectDir, "script_partial.json"),
		)...)
		errs = append(errs, removeDirs(filepath.Join(projectDir, "alignment_chunks"))...)
		errs = append(errs, removeGlobs(filepath.Join(projectDir, "audio_concat_*.txt"))...)
		fallthrough
	case StageAlign:
		errs = append(errs, removeFiles(
			filepath.Join(projectDir, "podcast_base.mp4"),
			filepath.Join(projectDir, "podcast_content.mp4"),
			filepath.Join(projectDir, "podcast_subtitles.ass"),
			filepath.Join(projectDir, "podcast_final.mp4"),
			filepath.Join(projectDir, "chat_script.pdf"),
			filepath.Join(projectDir, "youtube_publish.txt"),
		)...)
		errs = append(errs, removeGlobs(filepath.Join(projectDir, "youtube_transcript_*.srt"))...)
	case StageRender:
		errs = append(errs, removeFiles(
			filepath.Join(projectDir, "podcast_base.mp4"),
			filepath.Join(projectDir, "podcast_content.mp4"),
			filepath.Join(projectDir, "podcast_subtitles.ass"),
			filepath.Join(projectDir, "podcast_final.mp4"),
			filepath.Join(projectDir, "chat_script.pdf"),
			filepath.Join(projectDir, "youtube_publish.txt"),
		)...)
		errs = append(errs, removeGlobs(filepath.Join(projectDir, "youtube_transcript_*.srt"))...)
	case StageFinalize:
		errs = append(errs, removeFiles(
			filepath.Join(projectDir, "podcast_final.mp4"),
			filepath.Join(projectDir, "podcast_subtitles.ass"),
			filepath.Join(projectDir, "chat_script.pdf"),
			filepath.Join(projectDir, "youtube_publish.txt"),
		)...)
		errs = append(errs, removeGlobs(filepath.Join(projectDir, "youtube_transcript_*.srt"))...)
	case StagePersist:
		errs = append(errs, removeFiles(
			filepath.Join(projectDir, "chat_script.pdf"),
			filepath.Join(projectDir, "youtube_publish.txt"),
		)...)
		errs = append(errs, removeGlobs(filepath.Join(projectDir, "youtube_transcript_*.srt"))...)
	case StageUpload:
		// Upload rewrites only persisted downloads, not local render artifacts.
	}

	if stage == StageAlign && NormalizeTTSType(ttsType) != 1 {
		return fmt.Errorf("align stage is not supported for tts_type=%d", NormalizeTTSType(ttsType))
	}
	return errors.Join(errs...)
}

func removeFiles(paths ...string) []error {
	errs := make([]error, 0)
	for _, path := range paths {
		if strings.TrimSpace(path) == "" {
			continue
		}
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			errs = append(errs, fmt.Errorf("remove %s: %w", path, err))
		}
	}
	return errs
}

func removeDirs(paths ...string) []error {
	errs := make([]error, 0)
	for _, path := range paths {
		if strings.TrimSpace(path) == "" {
			continue
		}
		if err := os.RemoveAll(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			errs = append(errs, fmt.Errorf("remove %s: %w", path, err))
		}
	}
	return errs
}

func removeGlobs(pattern string) []error {
	if strings.TrimSpace(pattern) == "" {
		return nil
	}
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return []error{err}
	}
	errs := make([]error, 0, len(matches))
	for _, path := range matches {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			errs = append(errs, fmt.Errorf("remove %s: %w", path, err))
		}
	}
	return errs
}
