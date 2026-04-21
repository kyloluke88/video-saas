package podcast_compose_service

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	ffmpegpodcast "worker/services/podcast/render"
)

func TestBackgroundImagePathsForUsesOnlyFirstBackground(t *testing.T) {
	path, err := backgroundImagePathForRequest([]string{"a.png", "b.png", "c.png"})
	if err != nil {
		t.Fatalf("backgroundImagePathForRequest returned err: %v", err)
	}
	if path == "" {
		t.Fatalf("expected non-empty background path")
	}
}

func TestBackgroundImagePathsForRequiresBackgrounds(t *testing.T) {
	if _, err := backgroundImagePathForRequest(nil); err == nil {
		t.Fatalf("expected bg_img_filenames required error")
	}
}

func TestRenderUsesBaseVideoPath(t *testing.T) {
	pkgDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd returned err: %v", err)
	}
	repoRoot := filepath.Clean(filepath.Join(pkgDir, "..", "..", "..", ".."))
	assetsDir := filepath.Join(repoRoot, "worker", "assets")

	ffmpegWorkDir := filepath.Join(t.TempDir(), "ffmpeg")
	t.Setenv("worker.ffmpeg_work_dir", ffmpegWorkDir)
	t.Setenv("WORKER.FFMPEG_WORK_DIR", ffmpegWorkDir)
	t.Setenv("worker.worker_assets_dir", assetsDir)
	t.Setenv("WORKER.WORKER_ASSETS_DIR", assetsDir)

	projectID := "proj_render_base"
	projectDir := filepath.Join(ffmpegWorkDir, "projects", projectID)
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("MkdirAll returned err: %v", err)
	}
	dialoguePath := filepath.Join(projectDir, "dialogue.mp3")
	if err := os.WriteFile(dialoguePath, []byte("stub"), 0o644); err != nil {
		t.Fatalf("WriteFile returned err: %v", err)
	}
	scriptPath := filepath.Join(projectDir, "script_aligned.json")
	if err := os.WriteFile(scriptPath, []byte(`{"language":"ja","blocks":[]}`), 0o644); err != nil {
		t.Fatalf("WriteFile returned err: %v", err)
	}

	original := composeBaseVideoContext
	t.Cleanup(func() {
		composeBaseVideoContext = original
	})

	called := false
	composeBaseVideoContext = func(_ context.Context, input ffmpegpodcast.ComposeInput) error {
		called = true
		wantBase := filepath.Join(projectDir, "podcast_base.mp4")
		wantFinal := filepath.Join(projectDir, "podcast_final.mp4")
		if input.OutputPath != wantBase {
			t.Fatalf("expected render output path %s, got %s", wantBase, input.OutputPath)
		}
		if input.OutputPath == wantFinal {
			t.Fatalf("render should not write final output path: %s", input.OutputPath)
		}
		return nil
	}

	result, err := Render(context.Background(), ComposeInput{
		ProjectID:      projectID,
		Language:       "ja",
		BgImgFilenames: []string{"ja1.png"},
		Resolution:     "480p",
		DesignStyle:    1,
	})
	if err != nil {
		t.Fatalf("Render returned err: %v", err)
	}
	if !called {
		t.Fatalf("expected composeBaseVideoContext to be called")
	}
	if want := filepath.Join(projectDir, "podcast_base.mp4"); result.BaseVideoPath != want {
		t.Fatalf("expected BaseVideoPath %s, got %s", want, result.BaseVideoPath)
	}
}
