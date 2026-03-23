package podcast_audio_service

import (
	"strings"
	"testing"
)

func TestBuildGeminiBlockPrompt_ZHUsesFixedSpeakerBible(t *testing.T) {
	prompt := buildGeminiBlockPrompt("zh")

	for _, want := range []string{
		"Generate a natural two-speaker Mandarin Chinese learning podcast dialogue.",
		"Male speaker:",
		"Female speaker:",
		"Use stable voice characterization and keep the overall delivery consistent.",
		"Allow subtle natural warmth, light conversational reactions, and small emotional shading",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("expected zh prompt to contain %q, got %q", want, prompt)
		}
	}
	if strings.Contains(prompt, "Block purpose") {
		t.Fatalf("expected zh prompt to omit block purpose, got %q", prompt)
	}
}

func TestBuildGeminiBlockPrompt_JAUsesFixedSpeakerBible(t *testing.T) {
	prompt := buildGeminiBlockPrompt("ja")

	for _, want := range []string{
		"Generate a natural two-speaker Japanese learning podcast dialogue.",
		"Male speaker:",
		"Female speaker:",
		"Use stable voice characterization and keep the overall delivery consistent.",
		"Allow subtle warmth, light conversational responsiveness, and small emotional shading",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("expected ja prompt to contain %q, got %q", want, prompt)
		}
	}
	if strings.Contains(prompt, "Block purpose") {
		t.Fatalf("expected ja prompt to omit block purpose, got %q", prompt)
	}
}
