package workspace

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	services "worker/services"
)

func TestEnsureReplayProjectDirMissingSourceIsNonRetryable(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd failed: %v", err)
	}
	tempDir := t.TempDir()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("chdir temp dir failed: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(cwd)
	})

	err = EnsureReplayProjectDir("missing-project", "target-project")
	var nonRetryable services.NonRetryableError
	if !errors.As(err, &nonRetryable) {
		t.Fatalf("expected non-retryable error, got: %v", err)
	}
}

func TestLoadRequestPayloadMapMissingFileIsNonRetryable(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd failed: %v", err)
	}
	tempDir := t.TempDir()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("chdir temp dir failed: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(cwd)
	})

	_, err = LoadRequestPayloadMap("missing-project")
	var nonRetryable services.NonRetryableError
	if !errors.As(err, &nonRetryable) {
		t.Fatalf("expected non-retryable error, got: %v", err)
	}
}

func TestEnsureReplayProjectDirCopiesWhenSourceExists(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd failed: %v", err)
	}
	workDir := t.TempDir()
	if err := os.Chdir(workDir); err != nil {
		t.Fatalf("chdir temp dir failed: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(cwd)
	})

	sourceDir := filepath.Join(workDir, "projects", "source-project")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatalf("mkdir source dir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "request_payload.json"), []byte(`{"ok":true}`), 0o644); err != nil {
		t.Fatalf("write source file failed: %v", err)
	}

	if err := EnsureReplayProjectDir("source-project", "target-project"); err != nil {
		t.Fatalf("EnsureReplayProjectDir failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(workDir, "projects", "target-project", "request_payload.json")); err != nil {
		t.Fatalf("expected copied request_payload.json, got err: %v", err)
	}
}
