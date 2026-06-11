package podcast_audio_service

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"worker/pkg/googlecloud"
	services "worker/services"
	dto "worker/services/podcast/model"
)

func TestBuildRequestedBlockSet_DeduplicatesOneBasedInput(t *testing.T) {
	selected, err := buildRequestedBlockSet([]int{5, 1, 5, 3}, 6)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(selected) != 3 {
		t.Fatalf("expected 3 selected blocks, got %d", len(selected))
	}
	for _, index := range []int{0, 2, 4} {
		if _, ok := selected[index]; !ok {
			t.Fatalf("expected block index %d to be selected", index)
		}
	}
}

func TestBuildRequestedBlockSet_RejectsOutOfRangeValues(t *testing.T) {
	_, err := buildRequestedBlockSet([]int{2, 9}, 4)
	if err == nil {
		t.Fatalf("expected error when block_num exceeds block count")
	}
	var nonRetryable services.NonRetryableError
	if !errors.As(err, &nonRetryable) {
		t.Fatalf("expected non-retryable error, got %T", err)
	}
	if !strings.Contains(err.Error(), "block_nums out of range") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestMarkScriptLoadNonRetryable_MissingFile(t *testing.T) {
	err := markScriptLoadNonRetryable("/tmp/missing.json", os.ErrNotExist)
	var nonRetryable services.NonRetryableError
	if !errors.As(err, &nonRetryable) {
		t.Fatalf("expected non-retryable error, got %T", err)
	}
	if !strings.Contains(err.Error(), "script file not found") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestAssembleDialogue_InsertsGapBetweenBlocksEvenWhenSpeakerIsTheSame(t *testing.T) {
	base := dto.PodcastScript{
		Blocks: []dto.PodcastBlock{
			{
				BlockID: "block_001",
				Segments: []dto.PodcastSegment{
					{SegmentID: "seg_001", Speaker: "female", StartMS: 0, EndMS: 1000},
				},
			},
			{
				BlockID: "block_002",
				Segments: []dto.PodcastSegment{
					{SegmentID: "seg_002", Speaker: "female", StartMS: 0, EndMS: 800},
				},
			},
		},
	}
	results := []blockSynthesisResult{
		{
			AudioPath:    "/tmp/block_001.mp3",
			DurationMS:   1000,
			AlignedBlock: base.Blocks[0],
		},
		{
			AudioPath:    "/tmp/block_002.mp3",
			DurationMS:   800,
			AlignedBlock: base.Blocks[1],
		},
	}

	finalScript, concatPaths, totalMS, err := assembleDialogue(base, results, "/tmp/block_gap.wav", 280)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if totalMS != 2080 {
		t.Fatalf("expected total duration 2080ms, got %d", totalMS)
	}
	if len(concatPaths) != 3 {
		t.Fatalf("expected 3 concat paths, got %d", len(concatPaths))
	}
	if concatPaths[1] != "/tmp/block_gap.wav" {
		t.Fatalf("expected gap path in the middle, got %v", concatPaths)
	}
	if got := finalScript.Blocks[1].Segments[0].StartMS; got != 1280 {
		t.Fatalf("expected second block to start after gap at 1280ms, got %d", got)
	}
	if got := finalScript.Blocks[1].Segments[0].EndMS; got != 2080 {
		t.Fatalf("expected second block to end at 2080ms, got %d", got)
	}
}

func TestPersistGoogleTTSDebugArtifacts_WritesOneJsonPerBlock(t *testing.T) {
	dir := t.TempDir()
	req := googlecloud.SynthesizeConversationRequest{
		LanguageCode: "ja",
		Prompt:       "sample prompt",
		Turns: []googlecloud.ConversationTurn{
			{Speaker: "female", Text: "こんにちは"},
			{Speaker: "male", Text: "はい"},
		},
		MaleVoiceID:   "male_voice",
		FemaleVoiceID: "female_voice",
		SpeakingRate:  0.9,
	}

	if err := persistGoogleTTSDebugArtifacts(dir, "block_001", req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	path := filepath.Join(dir, "block_001.google_request.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("expected request json file to exist: %v", err)
	}
	content := string(raw)
	for _, want := range []string{
		"gemini-2.5-pro-preview-tts",
		"sample prompt",
		"generationConfig",
		"responseModalities",
		"\"speaker\": \"female\"",
		"\"speaker\": \"male\"",
		"female_voice",
		"male_voice",
		"voiceConfig",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("expected json to contain %q, got %s", want, content)
		}
	}
	for _, want := range []string{"盼盼", "老路", "ユイ", "アキラ"} {
		if strings.Contains(content, want) {
			t.Fatalf("expected json not to contain %q, got %s", want, content)
		}
	}
}

func TestCleanupGoogleTTSDebugArtifacts_RemovesDebugFilesOnly(t *testing.T) {
	projectDir := t.TempDir()
	ttsDebugDir := filepath.Join(projectDir, "tts_debug")
	blockStatesDir := filepath.Join(projectDir, "block_states")
	if err := os.MkdirAll(ttsDebugDir, 0o755); err != nil {
		t.Fatalf("failed to create tts_debug dir: %v", err)
	}
	if err := os.MkdirAll(blockStatesDir, 0o755); err != nil {
		t.Fatalf("failed to create block_states dir: %v", err)
	}

	keepPath := filepath.Join(blockStatesDir, "001_block_001.json")
	requestPath := filepath.Join(ttsDebugDir, "block_001.google_request.json")
	tempoPath := filepath.Join(blockStatesDir, "001_block_001.pre_tempo.wav")

	for path, content := range map[string]string{
		keepPath:    "{}",
		requestPath: "{}",
		tempoPath:   "audio",
	} {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("failed to write %s: %v", path, err)
		}
	}

	if err := cleanupGoogleTTSDebugArtifacts(projectDir); err != nil {
		t.Fatalf("cleanup returned error: %v", err)
	}
	if !fileExists(keepPath) {
		t.Fatalf("expected checkpoint json to remain")
	}
	if fileExists(requestPath) {
		t.Fatalf("expected request json to be removed")
	}
	if fileExists(tempoPath) {
		t.Fatalf("expected pre_tempo audio to be removed")
	}
}

func TestRemoveAlignedBlockArtifactsRemovesAudioAndState(t *testing.T) {
	projectDir := t.TempDir()
	artifacts, err := prepareAudioArtifacts(projectDir)
	if err != nil {
		t.Fatalf("prepareAudioArtifacts failed: %v", err)
	}

	audioPath := unitAudioPath(artifacts.blocksDir, 0, "block_001", "mp3")
	statePath := blockStatePath(artifacts.blockStatesDir, 0, "block_001")
	if err := os.WriteFile(audioPath, []byte("audio"), 0o644); err != nil {
		t.Fatalf("write audio failed: %v", err)
	}
	if err := os.WriteFile(statePath, []byte("{}"), 0o644); err != nil {
		t.Fatalf("write state failed: %v", err)
	}

	if err := removeAlignedBlockArtifacts(artifacts, 0, "block_001"); err != nil {
		t.Fatalf("removeAlignedBlockArtifacts failed: %v", err)
	}
	if fileExists(audioPath) {
		t.Fatalf("expected aligned audio to be removed")
	}
	if fileExists(statePath) {
		t.Fatalf("expected aligned state to be removed")
	}
}
