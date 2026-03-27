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
		"everyday spoken Mandarin Chinese",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("expected zh prompt to contain %q, got %q", want, prompt)
		}
	}
	for _, unwanted := range []string{"Japanese learning podcast", "Block purpose"} {
		if strings.Contains(prompt, unwanted) {
			t.Fatalf("expected zh prompt to omit %q, got %q", unwanted, prompt)
		}
	}
}

func TestBuildGeminiBlockPrompt_JAUsesFixedSpeakerBible(t *testing.T) {
	prompt := buildGeminiBlockPrompt("ja")

	for _, want := range []string{
		"Generate a natural two-speaker Japanese learning podcast dialogue.",
		"Male speaker:",
		"Female speaker:",
		"Use stable voice characterization and keep the overall delivery consistent.",
		"everyday spoken Japanese",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("expected ja prompt to contain %q, got %q", want, prompt)
		}
	}
	for _, unwanted := range []string{"Mandarin Chinese learning podcast", "Block purpose"} {
		if strings.Contains(prompt, unwanted) {
			t.Fatalf("expected ja prompt to omit %q, got %q", unwanted, prompt)
		}
	}
}
