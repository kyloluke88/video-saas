package pipeline

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInvalidateOutputsNeverDeletesProjectArtifacts(t *testing.T) {
	projectDir := withPracticalPipelineTempOutputs(t, "proj_all")
	mustWritePracticalPipelineFile(t, projectDir, "dialogue.wav")
	mustWritePracticalPipelineFile(t, projectDir, "script_aligned.json")
	mustWritePracticalPipelineFile(t, projectDir, "speaker_voice_map.json")
	mustWritePracticalPipelineFile(t, projectDir, "image_plan.json")
	mustWritePracticalPipelineFile(t, projectDir, "image_manifest.json")
	mustWritePracticalPipelineFile(t, projectDir, "practical_subtitles.ass")
	mustWritePracticalPipelineFile(t, projectDir, "practical_final.mp4")
	mustWritePracticalPipelineFile(t, projectDir, "youtube_publish.txt")
	mustWritePracticalPipelineFile(t, projectDir, "youtube_transcript_ja.srt")

	for _, stage := range []string{"generate", "align", "images", "render", "persist"} {
		if err := InvalidateOutputs("proj_all", 1, stage); err != nil {
			t.Fatalf("InvalidateOutputs(%s) returned err: %v", stage, err)
		}
	}

	assertPracticalPipelineExists(t, filepath.Join(projectDir, "dialogue.wav"))
	assertPracticalPipelineExists(t, filepath.Join(projectDir, "script_aligned.json"))
	assertPracticalPipelineExists(t, filepath.Join(projectDir, "speaker_voice_map.json"))
	assertPracticalPipelineExists(t, filepath.Join(projectDir, "image_plan.json"))
	assertPracticalPipelineExists(t, filepath.Join(projectDir, "image_manifest.json"))
	assertPracticalPipelineExists(t, filepath.Join(projectDir, "practical_subtitles.ass"))
	assertPracticalPipelineExists(t, filepath.Join(projectDir, "practical_final.mp4"))
	assertPracticalPipelineExists(t, filepath.Join(projectDir, "youtube_publish.txt"))
	assertPracticalPipelineExists(t, filepath.Join(projectDir, "youtube_transcript_ja.srt"))
}

func TestInvalidateAudioOutputsNeverDeletesProjectArtifacts(t *testing.T) {
	projectDir := withPracticalPipelineTempOutputs(t, "proj_audio")
	mustWritePracticalPipelineFile(t, projectDir, "dialogue.wav")
	mustWritePracticalPipelineFile(t, projectDir, "script_aligned.json")
	mustWritePracticalPipelineFile(t, projectDir, "speaker_voice_map.json")
	mustWritePracticalPipelineFile(t, projectDir, "practical_subtitles.ass")
	mustWritePracticalPipelineFile(t, projectDir, "practical_final.mp4")
	mustWritePracticalPipelineFile(t, projectDir, "youtube_publish.txt")
	mustWritePracticalPipelineFile(t, projectDir, "youtube_transcript_ja.srt")

	if err := InvalidateAudioOutputs("proj_audio", 1, "generate", nil, []int{7}); err != nil {
		t.Fatalf("InvalidateAudioOutputs(generate partial) returned err: %v", err)
	}
	if err := InvalidateAudioOutputs("proj_audio", 1, "align", []int{1}, nil); err != nil {
		t.Fatalf("InvalidateAudioOutputs(align partial) returned err: %v", err)
	}
	if err := InvalidateAudioOutputs("proj_audio", 1, "generate", nil, nil); err != nil {
		t.Fatalf("InvalidateAudioOutputs(generate full) returned err: %v", err)
	}

	assertPracticalPipelineExists(t, filepath.Join(projectDir, "dialogue.wav"))
	assertPracticalPipelineExists(t, filepath.Join(projectDir, "script_aligned.json"))
	assertPracticalPipelineExists(t, filepath.Join(projectDir, "speaker_voice_map.json"))
	assertPracticalPipelineExists(t, filepath.Join(projectDir, "practical_subtitles.ass"))
	assertPracticalPipelineExists(t, filepath.Join(projectDir, "practical_final.mp4"))
	assertPracticalPipelineExists(t, filepath.Join(projectDir, "youtube_publish.txt"))
	assertPracticalPipelineExists(t, filepath.Join(projectDir, "youtube_transcript_ja.srt"))
}

func withPracticalPipelineTempOutputs(t *testing.T, projectID string) string {
	t.Helper()
	tmpDir := t.TempDir()
	prevWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd failed: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir failed: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(prevWD)
	})

	projectDir := filepath.Join(tmpDir, "outputs", "projects", projectID)
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("mkdir project dir failed: %v", err)
	}
	return projectDir
}

func mustWritePracticalPipelineFile(t *testing.T, projectDir, name string) {
	t.Helper()
	path := filepath.Join(projectDir, name)
	if err := os.WriteFile(path, []byte(name), 0o644); err != nil {
		t.Fatalf("write %s failed: %v", path, err)
	}
}

func assertPracticalPipelineExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected %s to exist, stat err=%v", path, err)
	}
}

func assertPracticalPipelineMissing(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected %s to be removed, stat err=%v", path, err)
	}
}
