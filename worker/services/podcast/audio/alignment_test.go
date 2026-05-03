package podcast_audio_service

import (
	"testing"

	dto "worker/services/podcast/model"
)

func TestSpokenTextForMFA_ZHUsesCJKUnits(t *testing.T) {
	seg := dto.PodcastSegment{
		Text: "哎呀，三个闹钟！这样不会觉得更累吗？",
	}

	got := spokenTextForMFA("zh", seg)
	want := "哎 呀 三 个 闹 钟 这 样 不 会 觉 得 更 累 吗"
	if got != want {
		t.Fatalf("unexpected zh MFA transcript: got %q want %q", got, want)
	}
}

func TestBlockTranscriptForMFA_ZHSeparatesSegments(t *testing.T) {
	block := dto.PodcastBlock{
		Segments: []dto.PodcastSegment{
			{Text: "大家好！"},
			{Text: "我是盼盼。"},
		},
	}

	got := blockTranscriptForMFA("zh", block)
	want := "大 家 好\n我 是 盼 盼"
	if got != want {
		t.Fatalf("unexpected zh block transcript: got %q want %q", got, want)
	}
}

func TestSpokenTextForMFA_JAPreservesDisplayText(t *testing.T) {
	seg := dto.PodcastSegment{
		Text: "今日はいい天気ですね。",
	}

	got := spokenTextForMFA("ja", seg)
	want := "今日はいい天気ですね。"
	if got != want {
		t.Fatalf("unexpected ja MFA transcript: got %q want %q", got, want)
	}
}
