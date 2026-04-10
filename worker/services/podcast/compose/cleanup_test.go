package podcast_compose_service

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCleanupPodcastIntermediates(t *testing.T) {
	projectDir := t.TempDir()

	mustWrite := func(name string) {
		t.Helper()
		path := filepath.Join(projectDir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", path, err)
		}
		if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}

	for _, name := range []string{
		"podcast_base.mp4",
		"podcast_content.mp4",
		"podcast_subtitles.ass",
		"script_partial.json",
		"audio_concat_111.txt",
		"audio_concat_222.txt",
	} {
		mustWrite(name)
	}
	if err := os.MkdirAll(filepath.Join(projectDir, "alignment_chunks"), 0o755); err != nil {
		t.Fatalf("mkdir alignment_chunks: %v", err)
	}
	mustWrite(filepath.Join("alignment_chunks", "debug.txt"))

	for _, name := range []string{
		"podcast_final.mp4",
		"dialogue.mp3",
		"script_aligned.json",
		"request_payload.json",
	} {
		mustWrite(name)
	}

	if err := cleanupPodcastIntermediates(projectDir); err != nil {
		t.Fatalf("cleanupPodcastIntermediates: %v", err)
	}

	for _, name := range []string{
		"podcast_base.mp4",
		"podcast_content.mp4",
		"podcast_subtitles.ass",
		"script_partial.json",
		"audio_concat_111.txt",
		"audio_concat_222.txt",
	} {
		if _, err := os.Stat(filepath.Join(projectDir, name)); !os.IsNotExist(err) {
			t.Fatalf("expected %s to be removed, stat err=%v", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(projectDir, "alignment_chunks")); !os.IsNotExist(err) {
		t.Fatalf("expected alignment_chunks to be removed, stat err=%v", err)
	}

	for _, name := range []string{
		"podcast_final.mp4",
		"dialogue.mp3",
		"script_aligned.json",
		"request_payload.json",
	} {
		if _, err := os.Stat(filepath.Join(projectDir, name)); err != nil {
			t.Fatalf("expected %s to remain, stat err=%v", name, err)
		}
	}
}
