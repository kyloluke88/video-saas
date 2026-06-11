package podcast_audio_service

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	services "worker/services"
	dto "worker/services/podcast/model"
)

func TestValidateAlignedTimeline_NonMonotonicIsNonRetryable(t *testing.T) {
	script := dto.PodcastScript{
		Blocks: []dto.PodcastBlock{{
			BlockID: "block_001",
			Segments: []dto.PodcastSegment{
				{SegmentID: "seg_001", StartMS: 100, EndMS: 300},
				{SegmentID: "seg_002", StartMS: 250, EndMS: 400},
			},
		}},
	}

	err := validateAlignedTimeline(script, "")
	if err == nil {
		t.Fatalf("expected non-monotonic timeline error")
	}
	var nonRetryable services.NonRetryableError
	if !errors.As(err, &nonRetryable) {
		t.Fatalf("expected non-retryable error, got %T", err)
	}
}

func TestValidateAlignedTimeline_InvalidSegmentWindowIsNonRetryable(t *testing.T) {
	script := dto.PodcastScript{
		Blocks: []dto.PodcastBlock{{
			BlockID: "block_001",
			Segments: []dto.PodcastSegment{
				{SegmentID: "seg_001", StartMS: 100, EndMS: 100},
			},
		}},
	}

	err := validateAlignedTimeline(script, "")
	if err == nil {
		t.Fatalf("expected invalid timeline error")
	}
	var nonRetryable services.NonRetryableError
	if !errors.As(err, &nonRetryable) {
		t.Fatalf("expected non-retryable error, got %T", err)
	}
}

func TestBlockCheckpointCompleteRejectsMissingTokenTiming(t *testing.T) {
	dir := t.TempDir()
	audioPath := filepath.Join(dir, "001_block_001.wav")
	if err := os.WriteFile(audioPath, []byte("audio"), 0o644); err != nil {
		t.Fatalf("failed to write audio file: %v", err)
	}

	state := blockCheckpoint{
		Block: dto.PodcastBlock{
			Segments: []dto.PodcastSegment{
				{
					SegmentID: "seg_001",
					StartMS:   0,
					EndMS:     1000,
					Tokens: []dto.PodcastToken{
						{Char: "你"},
					},
				},
			},
		},
		DurationMS: 1000,
		Tempo:      0.78,
	}

	if blockCheckpointComplete("zh", state, audioPath, 0.78, false) {
		t.Fatalf("expected checkpoint with missing token timing to be rejected")
	}
}

func TestBlockCheckpointCompleteAcceptsJapaneseHighlightTiming(t *testing.T) {
	dir := t.TempDir()
	audioPath := filepath.Join(dir, "001_block_001.wav")
	if err := os.WriteFile(audioPath, []byte("audio"), 0o644); err != nil {
		t.Fatalf("failed to write audio file: %v", err)
	}

	state := blockCheckpoint{
		Block: dto.PodcastBlock{
			Segments: []dto.PodcastSegment{
				{
					SegmentID: "seg_001",
					StartMS:   0,
					EndMS:     1000,
					HighlightSpans: []dto.PodcastHighlightSpan{
						{StartIndex: 0, EndIndex: 1, StartMS: 10, EndMS: 100},
					},
				},
			},
		},
		DurationMS: 1000,
		Tempo:      0.78,
	}

	if !blockCheckpointComplete("ja", state, audioPath, 0.78, false) {
		t.Fatalf("expected checkpoint with highlight timings to be accepted")
	}
}

func TestBlockCheckpointCompleteRejectsTempoMismatch(t *testing.T) {
	dir := t.TempDir()
	audioPath := filepath.Join(dir, "001_block_001.wav")
	if err := os.WriteFile(audioPath, []byte("audio"), 0o644); err != nil {
		t.Fatalf("failed to write audio file: %v", err)
	}

	state := blockCheckpoint{
		Block: dto.PodcastBlock{
			Segments: []dto.PodcastSegment{
				{
					SegmentID: "seg_001",
					StartMS:   0,
					EndMS:     1000,
					Tokens: []dto.PodcastToken{
						{Char: "你", StartMS: 10, EndMS: 100},
					},
				},
			},
		},
		DurationMS: 1000,
		Tempo:      0.78,
	}

	if blockCheckpointComplete("zh", state, audioPath, 0.9, false) {
		t.Fatalf("expected tempo mismatch to reject reuse")
	}
}

func TestLoadCachedScriptForAlignmentReadsAlignedScript(t *testing.T) {
	projectDir := t.TempDir()
	aligned := `{
		"language":"ja",
		"title":"aligned title",
		"blocks":[
			{
				"block_id":"block_001",
				"segments":[
					{
						"segment_id":"seg_001",
						"text":"こんにちは",
						"tokens":[{"char":"今日","reading":"きょう","start_ms":10,"end_ms":100}],
						"highlight_spans":[{"start_index":0,"end_index":1,"start_ms":10,"end_ms":100}],
						"start_ms":0,
						"end_ms":300
					}
				]
			}
		]
	}`
	input := `{
		"language":"ja",
		"title":"input title"
	}`

	if err := os.WriteFile(projectScriptAlignedPath(projectDir), []byte(aligned), 0o644); err != nil {
		t.Fatalf("write script_aligned.json failed: %v", err)
	}
	if err := os.WriteFile(projectScriptInputPath(projectDir), []byte(input), 0o644); err != nil {
		t.Fatalf("write script_input.json failed: %v", err)
	}

	script, err := loadCachedScriptForAlignment(projectDir, "ja")
	if err != nil {
		t.Fatalf("loadCachedScriptForAlignment failed: %v", err)
	}
	if got, want := script.Title, "aligned title"; got != want {
		t.Fatalf("expected aligned script title %q, got %q", want, got)
	}
}

func TestLoadCachedScriptForAlignmentRestoresStableBlockIDsFromArtifacts(t *testing.T) {
	projectDir := t.TempDir()
	aligned := `{
		"language":"ja",
		"youtube":{
			"chapters":[
				{"chapter_id":"ch_001","title":"opening","block_ids":["block_001.1"]}
			]
		},
		"blocks":[
			{
				"block_id":"block_001.1",
				"segments":[
					{
						"segment_id":"seg_001",
						"text":"こんにちは",
						"tokens":[{"char":"今","reading":"いま","start_ms":10,"end_ms":100}],
						"highlight_spans":[{"start_index":0,"end_index":1,"start_ms":10,"end_ms":100}],
						"start_ms":0,
						"end_ms":300
					}
				]
			}
		]
	}`
	if err := os.WriteFile(projectScriptAlignedPath(projectDir), []byte(aligned), 0o644); err != nil {
		t.Fatalf("write script_aligned.json failed: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(projectDir, "blocks"), 0o755); err != nil {
		t.Fatalf("mkdir blocks failed: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(projectDir, "block_states"), 0o755); err != nil {
		t.Fatalf("mkdir block_states failed: %v", err)
	}
	if err := os.WriteFile(unitAudioPath(filepath.Join(projectDir, "blocks"), 0, "block_001", "wav"), []byte("audio"), 0o644); err != nil {
		t.Fatalf("write block audio failed: %v", err)
	}
	state := blockCheckpoint{
		Block: dto.PodcastBlock{
			BlockID: "block_001",
			Segments: []dto.PodcastSegment{{
				SegmentID: "seg_001",
				Text:      "こんにちは",
				StartMS:   0,
				EndMS:     300,
				HighlightSpans: []dto.PodcastHighlightSpan{
					{StartIndex: 0, EndIndex: 1, StartMS: 10, EndMS: 100},
				},
			}},
		},
		DurationMS: 300,
	}
	if err := writeJSON(blockStatePath(filepath.Join(projectDir, "block_states"), 0, "block_001"), state); err != nil {
		t.Fatalf("write block state failed: %v", err)
	}

	script, err := loadCachedScriptForAlignment(projectDir, "ja")
	if err != nil {
		t.Fatalf("loadCachedScriptForAlignment failed: %v", err)
	}
	if got, want := script.Blocks[0].BlockID, "block_001"; got != want {
		t.Fatalf("expected restored block id %q, got %q", want, got)
	}
	if got, want := script.YouTube.Chapters[0].BlockIDs[0], "block_001"; got != want {
		t.Fatalf("expected restored chapter block id %q, got %q", want, got)
	}
}

func TestFinalizeAlignedScriptPreservesExistingBlockIDs(t *testing.T) {
	projectDir := t.TempDir()
	alignedPath := filepath.Join(projectDir, "script_aligned.json")
	script := dto.PodcastScript{
		Language: "ja",
		YouTube: dto.PodcastYouTube{
			Chapters: []dto.PodcastYouTubeChapter{
				{ChapterID: "ch_001", BlockIDs: []string{"block_001"}},
			},
		},
		Blocks: []dto.PodcastBlock{
			{
				ChapterID: "ch_001",
				BlockID:   "block_001",
				Segments: []dto.PodcastSegment{
					{
						SegmentID: "seg_001",
						Text:      "こんにちは",
						StartMS:   0,
						EndMS:     300,
						HighlightSpans: []dto.PodcastHighlightSpan{
							{StartIndex: 0, EndIndex: 1, StartMS: 10, EndMS: 100},
						},
					},
				},
			},
		},
	}

	got, err := finalizeAlignedScript("proj", alignedPath, "", script)
	if err != nil {
		t.Fatalf("finalizeAlignedScript failed: %v", err)
	}
	if got.Blocks[0].BlockID != "block_001" {
		t.Fatalf("expected block id to be preserved, got %q", got.Blocks[0].BlockID)
	}
	if got.YouTube.Chapters[0].BlockIDs[0] != "block_001" {
		t.Fatalf("expected chapter block id to be preserved, got %q", got.YouTube.Chapters[0].BlockIDs[0])
	}
}

func TestBuildProvisionalAlignedScriptCreatesHeuristicTimeline(t *testing.T) {
	projectDir := t.TempDir()
	artifacts, err := prepareAudioArtifacts(projectDir)
	if err != nil {
		t.Fatalf("prepareAudioArtifacts failed: %v", err)
	}

	script := dto.PodcastScript{
		Language: "zh",
		Blocks: []dto.PodcastBlock{
			{
				BlockID: "block_001",
				Segments: []dto.PodcastSegment{{
					SegmentID: "seg_001",
					Text:      "你好世界",
					Tokens: []dto.PodcastToken{
						{Char: "你"},
						{Char: "好"},
						{Char: "世"},
						{Char: "界"},
					},
				}},
			},
			{
				BlockID: "block_002",
				Segments: []dto.PodcastSegment{{
					SegmentID: "seg_002",
					Text:      "再见朋友",
					Tokens: []dto.PodcastToken{
						{Char: "再"},
						{Char: "见"},
						{Char: "朋"},
						{Char: "友"},
					},
				}},
			},
		},
	}

	results := []blockSynthesisResult{
		{
			AudioPath:    unitAudioPath(artifacts.blocksDir, 0, "block_001", "mp3"),
			DurationMS:   1000,
			AlignedBlock: script.Blocks[0],
		},
		{
			AudioPath:    unitAudioPath(artifacts.blocksDir, 1, "block_002", "mp3"),
			DurationMS:   1000,
			AlignedBlock: script.Blocks[1],
		},
	}

	got, err := buildProvisionalAlignedScript("zh", script, results, artifacts.blockGapPath, 280)
	if err != nil {
		t.Fatalf("buildProvisionalAlignedScript failed: %v", err)
	}
	if len(got.Blocks) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(got.Blocks))
	}
	first := got.Blocks[0].Segments[0]
	second := got.Blocks[1].Segments[0]
	if first.EndMS <= first.StartMS {
		t.Fatalf("expected first segment to have heuristic timing, got start=%d end=%d", first.StartMS, first.EndMS)
	}
	if second.StartMS != 1280 {
		t.Fatalf("expected second segment to start after first block and gap at 1280ms, got %d", second.StartMS)
	}
	if second.EndMS <= second.StartMS {
		t.Fatalf("expected second segment to have heuristic timing, got start=%d end=%d", second.StartMS, second.EndMS)
	}
}

func TestLoadBlockCheckpointReadsCheckpointFile(t *testing.T) {
	projectDir := t.TempDir()
	artifacts, err := prepareAudioArtifacts(projectDir)
	if err != nil {
		t.Fatalf("prepareAudioArtifacts failed: %v", err)
	}

	state := blockCheckpoint{
		Block: dto.PodcastBlock{
			BlockID: "block_001",
			Segments: []dto.PodcastSegment{
				{
					SegmentID: "seg_001",
					StartMS:   0,
					EndMS:     1000,
					Tokens: []dto.PodcastToken{
						{Char: "你", StartMS: 10, EndMS: 100},
						{Char: "好", StartMS: 120, EndMS: 220},
					},
				},
			},
		},
		DurationMS: 1000,
	}
	path := blockStatePath(artifacts.blockStatesDir, 0, state.Block.BlockID)
	if err := writeJSON(path, state); err != nil {
		t.Fatalf("write legacy checkpoint failed: %v", err)
	}

	got, ok, err := loadBlockCheckpoint(artifacts.blockStatesDir, 0, state.Block.BlockID)
	if err != nil {
		t.Fatalf("loadBlockCheckpoint failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected legacy checkpoint to load")
	}
	if got.DurationMS != 1000 {
		t.Fatalf("expected duration 1000, got %d", got.DurationMS)
	}
	if got.Block.Segments[0].Tokens[0].StartMS != 10 {
		t.Fatalf("expected token timing to load, got %+v", got.Block.Segments[0].Tokens[0])
	}
}

func TestPersistBlockCheckpointOverwritesCheckpointFile(t *testing.T) {
	projectDir := t.TempDir()
	artifacts, err := prepareAudioArtifacts(projectDir)
	if err != nil {
		t.Fatalf("prepareAudioArtifacts failed: %v", err)
	}

	original := dto.PodcastBlock{
		BlockID: "block_001",
		Segments: []dto.PodcastSegment{
			{
				SegmentID: "seg_001",
				Text:      "你好",
				StartMS:   0,
				EndMS:     1000,
				Tokens: []dto.PodcastToken{
					{Char: "你", StartMS: 10, EndMS: 100},
					{Char: "好", StartMS: 120, EndMS: 220},
				},
			},
		},
	}
	if err := persistBlockCheckpoint(artifacts.blockStatesDir, 0, original, 1000, 0.78); err != nil {
		t.Fatalf("persistBlockCheckpoint failed: %v", err)
	}

	updated := original
	updated.Segments = []dto.PodcastSegment{
		{
			SegmentID: "seg_001",
			Text:      "再见",
			StartMS:   0,
			EndMS:     1500,
			Tokens: []dto.PodcastToken{
				{Char: "再", StartMS: 10, EndMS: 200},
				{Char: "见", StartMS: 220, EndMS: 420},
			},
		},
	}
	if err := persistBlockCheckpoint(artifacts.blockStatesDir, 0, updated, 1500, 0.9); err != nil {
		t.Fatalf("persistBlockCheckpoint overwrite failed: %v", err)
	}

	got, ok, err := loadBlockCheckpoint(artifacts.blockStatesDir, 0, updated.BlockID)
	if err != nil {
		t.Fatalf("loadBlockCheckpoint failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected checkpoint to reload")
	}
	if got.DurationMS != 1500 {
		t.Fatalf("expected duration 1500 after reload, got %d", got.DurationMS)
	}
	if got.Block.Segments[0].Text != "再见" {
		t.Fatalf("expected updated checkpoint text, got %q", got.Block.Segments[0].Text)
	}
	if got.Tempo != 0.9 {
		t.Fatalf("expected updated tempo 0.9, got %v", got.Tempo)
	}
}
