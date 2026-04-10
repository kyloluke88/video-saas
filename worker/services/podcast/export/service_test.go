package podcast_export_service

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateFromProjectDirCreatesExportFiles(t *testing.T) {
	assetsDir, err := filepath.Abs(filepath.Join("..", "..", "..", "assets"))
	if err != nil {
		t.Fatalf("resolve assets dir failed: %v", err)
	}
	sourceDir, err := filepath.Abs(filepath.Join("..", "..", "..", "outputs", "projects", "zh_podcast_20260401165006_json"))
	if err != nil {
		t.Fatalf("resolve source dir failed: %v", err)
	}

	t.Setenv("WORKER_ASSETS_DIR", assetsDir)

	projectDir := filepath.Join(t.TempDir(), "zh_podcast_20260401165006")
	if err := copyDir(sourceDir, projectDir); err != nil {
		t.Fatalf("copy fixture project failed: %v", err)
	}

	result, err := GenerateFromProjectDir(projectDir, "zh_podcast_20260401165006")
	if err != nil {
		t.Fatalf("GenerateFromProjectDir failed: %v", err)
	}

	for _, path := range []string{result.PDFPath, result.YouTubePublishPath, result.YouTubeTranscriptPath} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat export file failed path=%s err=%v", path, err)
		}
		if info.Size() == 0 {
			t.Fatalf("expected export file to be non-empty: %s", path)
		}
	}

	publishRaw, err := os.ReadFile(result.YouTubePublishPath)
	if err != nil {
		t.Fatalf("read publish file failed: %v", err)
	}
	if !strings.Contains(string(publishRaw), "https://podcast.lucayo.com/podcast/scripts/what-panics-first-timers-in-china") {
		t.Fatalf("expected publish file to contain canonical page url, got %q", string(publishRaw))
	}
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		targetPath := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(targetPath, 0o755)
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(targetPath, raw, info.Mode())
	})
}
