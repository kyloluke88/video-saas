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
