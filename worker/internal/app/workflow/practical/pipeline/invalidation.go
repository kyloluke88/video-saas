package pipeline

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func InvalidateOutputs(projectID string, startFrom string) error {
	stage, ok := ParseStage(startFrom)
	if !ok {
		return fmt.Errorf("unsupported practical stage: %s", strings.TrimSpace(startFrom))
	}

	projectDir := practicalProjectDir(projectID)
	var errs []error

	switch stage {
	case StageGenerate, StageAlign:
		errs = append(errs, removeFiles(
			filepath.Join(projectDir, "dialogue.wav"),
			filepath.Join(projectDir, "script_aligned.json"),
			filepath.Join(projectDir, "practical_subtitles.ass"),
			filepath.Join(projectDir, "practical_final.mp4"),
			filepath.Join(projectDir, "youtube_publish.txt"),
		)...)
		errs = append(errs, removeGlobs(filepath.Join(projectDir, "youtube_transcript_*.srt"))...)
	case StageImages:
		errs = append(errs, removeFiles(
			filepath.Join(projectDir, "image_plan.json"),
			filepath.Join(projectDir, "image_manifest.json"),
			filepath.Join(projectDir, "practical_subtitles.ass"),
			filepath.Join(projectDir, "practical_final.mp4"),
			filepath.Join(projectDir, "youtube_publish.txt"),
		)...)
		errs = append(errs, removeGlobs(filepath.Join(projectDir, "youtube_transcript_*.srt"))...)
	case StageRender:
		errs = append(errs, removeFiles(
			filepath.Join(projectDir, "practical_subtitles.ass"),
			filepath.Join(projectDir, "practical_final.mp4"),
			filepath.Join(projectDir, "youtube_publish.txt"),
		)...)
		errs = append(errs, removeGlobs(filepath.Join(projectDir, "youtube_transcript_*.srt"))...)
	case StagePersist:
		errs = append(errs, removeFiles(filepath.Join(projectDir, "youtube_publish.txt"))...)
		errs = append(errs, removeGlobs(filepath.Join(projectDir, "youtube_transcript_*.srt"))...)
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
