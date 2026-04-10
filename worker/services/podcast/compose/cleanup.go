package podcast_compose_service

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

var podcastIntermediateFiles = []string{
	"podcast_base.mp4",
	"podcast_content.mp4",
	"podcast_subtitles.ass",
	"script_partial.json",
}

func cleanupPodcastIntermediates(projectDir string) error {
	if projectDir == "" {
		return nil
	}

	var errs []error
	for _, name := range podcastIntermediateFiles {
		if err := removeFileIfExists(filepath.Join(projectDir, name)); err != nil {
			errs = append(errs, err)
		}
	}
	if err := removeDirIfExists(filepath.Join(projectDir, "alignment_chunks")); err != nil {
		errs = append(errs, err)
	}

	concatFiles, err := filepath.Glob(filepath.Join(projectDir, "audio_concat_*.txt"))
	if err != nil {
		errs = append(errs, err)
	} else {
		sort.Strings(concatFiles)
		for _, path := range concatFiles {
			if err := removeFileIfExists(path); err != nil {
				errs = append(errs, err)
			}
		}
	}

	return errors.Join(errs...)
}

func removeFileIfExists(path string) error {
	if path == "" {
		return nil
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove %s: %w", path, err)
	}
	return nil
}

func removeDirIfExists(path string) error {
	if path == "" {
		return nil
	}
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("stat %s: %w", path, err)
	}
	if err := os.RemoveAll(path); err != nil {
		return fmt.Errorf("remove %s: %w", path, err)
	}
	return nil
}
