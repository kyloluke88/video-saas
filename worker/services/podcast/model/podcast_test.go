package model

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestPodcastSegmentUnmarshalUsesNestedEnglishTranslation(t *testing.T) {
	raw := []byte(`{
		"segment_id":"seg_001",
		"text":"こんにちは",
		"translations":{
			"en":"nested english",
			"ja":"こんにちは"
		}
	}`)

	var seg PodcastSegment
	if err := json.Unmarshal(raw, &seg); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if got, want := seg.EnglishTranslation(), "nested english"; got != want {
		t.Fatalf("unexpected english translation: got %q want %q", got, want)
	}
	if got, ok := seg.Translations["en"]; !ok || got != "nested english" {
		t.Fatalf("unexpected translations.en: %q ok=%v", got, ok)
	}
}

func TestPodcastSegmentMarshalOmitsTopLevelEN(t *testing.T) {
	seg := PodcastSegment{
		SegmentID: "seg_001",
		Text:      "こんにちは",
		Translations: map[string]string{
			"en": "hello",
			"ja": "こんにちは",
		},
	}

	raw, err := json.Marshal(seg)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var payload map[string]json.RawMessage
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("decode payload failed: %v", err)
	}
	if _, ok := payload["en"]; ok {
		t.Fatalf("expected top-level en to be omitted, got %s", string(raw))
	}
	translationsRaw, ok := payload["translations"]
	if !ok {
		t.Fatalf("expected translations to be present, got %s", string(raw))
	}
	var translations map[string]string
	if err := json.Unmarshal(translationsRaw, &translations); err != nil {
		t.Fatalf("decode translations failed: %v", err)
	}
	if got, want := translations["en"], "hello"; got != want {
		t.Fatalf("unexpected translations.en: got %q want %q", got, want)
	}
	if got, want := translations["ja"], "こんにちは"; got != want {
		t.Fatalf("unexpected translations.ja: got %q want %q", got, want)
	}
}

func TestPodcastAudioGeneratePayloadMarshalKeepsZeroRunMode(t *testing.T) {
	raw, err := json.Marshal(PodcastAudioGeneratePayload{
		ProjectID:      "proj_001",
		Lang:           "zh",
		ContentProfile: "podcast",
		ScriptFilename: "script.json",
		RunMode:        0,
	})
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	if !strings.Contains(string(raw), `"run_mode":0`) {
		t.Fatalf("expected run_mode to be marshaled, got %s", string(raw))
	}
}

func TestPodcastComposePayloadMarshalKeepsZeroRunMode(t *testing.T) {
	raw, err := json.Marshal(PodcastComposePayload{
		ProjectID:      "proj_001",
		Lang:           "zh",
		RunMode:        0,
		BgImgFilenames: []string{"bg.png"},
	})
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	if !strings.Contains(string(raw), `"run_mode":0`) {
		t.Fatalf("expected run_mode to be marshaled, got %s", string(raw))
	}
}

func TestPodcastScriptRenumberStructureIDsPreservesStableBlockIDs(t *testing.T) {
	script := PodcastScript{
		YouTube: PodcastYouTube{
			Chapters: []PodcastYouTubeChapter{
				{ChapterID: "opening", Title: "Opening"},
				{ChapterID: "closing", Title: "Closing"},
			},
		},
		Blocks: []PodcastBlock{
			{
				ChapterID: "opening",
				BlockID:   "block_001",
				Segments: []PodcastSegment{
					{SegmentID: "seg_old_1"},
				},
			},
			{
				ChapterID: "closing",
				BlockID:   "summary_cta",
				Segments: []PodcastSegment{
					{SegmentID: "seg_old_2"},
				},
			},
		},
	}

	script.RenumberStructureIDs()

	if got, want := script.Blocks[0].BlockID, "block_001"; got != want {
		t.Fatalf("expected first block id %q, got %q", want, got)
	}
	if got, want := script.Blocks[1].BlockID, "summary_cta"; got != want {
		t.Fatalf("expected summary block id %q, got %q", want, got)
	}
	if got, want := script.YouTube.Chapters[0].BlockIDs[0], "block_001"; got != want {
		t.Fatalf("expected first chapter block id %q, got %q", want, got)
	}
	if got, want := script.YouTube.Chapters[1].BlockIDs[0], "summary_cta"; got != want {
		t.Fatalf("expected second chapter block id %q, got %q", want, got)
	}
	if got, want := script.Blocks[0].Segments[0].SegmentID, "seg_001"; got != want {
		t.Fatalf("expected first segment id %q, got %q", want, got)
	}
	if got, want := script.Blocks[1].Segments[0].SegmentID, "seg_002"; got != want {
		t.Fatalf("expected second segment id %q, got %q", want, got)
	}
}

func TestPodcastScriptRenumberStructureIDsNormalizesLegacyDottedBlockIDs(t *testing.T) {
	script := PodcastScript{
		Blocks: []PodcastBlock{
			{BlockID: "block_001.1"},
			{BlockID: "summary_cta.14"},
			{BlockID: ""},
		},
	}

	script.RenumberStructureIDs()

	if got, want := script.Blocks[0].BlockID, "block_001"; got != want {
		t.Fatalf("expected normalized block id %q, got %q", want, got)
	}
	if got, want := script.Blocks[1].BlockID, "summary_cta"; got != want {
		t.Fatalf("expected normalized summary block id %q, got %q", want, got)
	}
	if got, want := script.Blocks[2].BlockID, "block_003"; got != want {
		t.Fatalf("expected fallback block id %q, got %q", want, got)
	}
}
