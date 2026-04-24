package podcast_export_service

import (
	"strings"
	"testing"

	dto "worker/services/podcast/model"
)

func TestBuildYouTubeTranscriptSRT(t *testing.T) {
	script := dto.PodcastScript{
		Language: "zh",
		Segments: []dto.PodcastSegment{
			{
				SegmentID: "seg_001",
				Text:      "大家好，欢迎来到我们的日常中文频道。",
				Translations: map[string]string{
					"en": "Hello everyone, welcome to our daily Chinese channel.",
				},
				StartMS: 0,
				EndMS:   2450,
			},
			{
				SegmentID: "seg_002",
				Text:      "今天我们来聊一聊年轻人的婚恋观。",
				Translations: map[string]string{
					"en": "Today we are going to talk about young people's views on marriage and relationships.",
				},
				StartMS: 2450,
				EndMS:   5870,
			},
		},
	}

	got := buildYouTubeTranscriptSRT(script)
	wantParts := []string{
		"1\n00:00:00,000 --> 00:00:02,450\n大家好，欢迎来到我们的日常中文频道。",
		"2\n00:00:02,450 --> 00:00:05,870\n今天我们来聊一聊年轻人的婚恋观。",
	}
	for _, part := range wantParts {
		if !strings.Contains(got, part) {
			t.Fatalf("expected transcript to contain %q, got %q", part, got)
		}
	}
}

func TestBuildYouTubeEnglishTranscriptSRT(t *testing.T) {
	script := dto.PodcastScript{
		Language: "zh",
		Segments: []dto.PodcastSegment{
			{
				SegmentID: "seg_001",
				Text:      "大家好，欢迎来到我们的日常中文频道。",
				Translations: map[string]string{
					"en": "Hello everyone, welcome to our daily Chinese channel.",
				},
				StartMS: 0,
				EndMS:   2450,
			},
			{
				SegmentID: "seg_002",
				Text:      "今天我们来聊一聊年轻人的婚恋观。",
				Translations: map[string]string{
					"en": "Today we are going to talk about young people's views on marriage and relationships.",
				},
				StartMS: 2450,
				EndMS:   5870,
			},
		},
	}

	got := buildYouTubeEnglishTranscriptSRT(script)
	wantParts := []string{
		"1\n00:00:00,000 --> 00:00:02,450\nHello everyone, welcome to our daily Chinese channel.",
		"2\n00:00:02,450 --> 00:00:05,870\nToday we are going to talk about young people's views on marriage and relationships.",
	}
	for _, part := range wantParts {
		if !strings.Contains(got, part) {
			t.Fatalf("expected english transcript to contain %q, got %q", part, got)
		}
	}
}

func TestBuildYouTubeTranscriptArtifactsIncludesAllTranslationLanguages(t *testing.T) {
	script := dto.PodcastScript{
		Language: "zh",
		Segments: []dto.PodcastSegment{
			{
				SegmentID: "seg_001",
				Text:      "大家好，欢迎来到我们的日常中文频道。",
				Translations: map[string]string{
					"en":     "Hello everyone, welcome to our daily Chinese channel.",
					"es-419": "Hola a todos, bienvenidos a nuestro canal diario de chino.",
					"vi":     "Xin chào mọi người, chào mừng đến với kênh tiếng Trung hằng ngày của chúng tôi.",
					"ja":     "みなさん、こんにちは。私たちの日常中国語チャンネルへようこそ。",
					"pt-BR":  "Olá pessoal, bem-vindos ao nosso canal diário de chinês.",
				},
				StartMS: 0,
				EndMS:   2450,
			},
		},
	}

	artifacts := buildYouTubeTranscriptArtifacts(script)
	got := make(map[string]string, len(artifacts))
	for _, artifact := range artifacts {
		got[artifact.filename] = artifact.content
	}

	wantFiles := []string{
		"youtube_transcript.srt",
		"youtube_transcript_en.srt",
		"youtube_transcript_zh.srt",
		"youtube_transcript_es-419.srt",
		"youtube_transcript_vi.srt",
		"youtube_transcript_ja.srt",
		"youtube_transcript_pt-BR.srt",
	}
	for _, filename := range wantFiles {
		content, ok := got[filename]
		if !ok {
			t.Fatalf("expected artifact %s to be generated, got %#v", filename, got)
		}
		if strings.TrimSpace(content) == "" {
			t.Fatalf("expected artifact %s to be non-empty", filename)
		}
	}

	if !strings.Contains(got["youtube_transcript_ja.srt"], "みなさん、こんにちは。私たちの日常中国語チャンネルへようこそ。") {
		t.Fatalf("expected japanese transcript content, got %q", got["youtube_transcript_ja.srt"])
	}
	if !strings.Contains(got["youtube_transcript_en.srt"], "Hello everyone, welcome to our daily Chinese channel.") {
		t.Fatalf("expected english transcript content, got %q", got["youtube_transcript_en.srt"])
	}
	if !strings.Contains(got["youtube_transcript_zh.srt"], "大家好，欢迎来到我们的日常中文频道。") {
		t.Fatalf("expected chinese transcript content, got %q", got["youtube_transcript_zh.srt"])
	}
}

func TestBuildYouTubeTranscriptSRTWithLeadIn(t *testing.T) {
	script := dto.PodcastScript{
		Language: "ja",
		Segments: []dto.PodcastSegment{
			{
				SegmentID: "seg_001",
				Text:      "みなさん、こんにちは。",
				StartMS:   100,
				EndMS:     2100,
			},
		},
	}

	got := buildYouTubeTranscriptSRTWithLeadIn(script, 4070)
	want := "1\n00:00:04,170 --> 00:00:06,170\nみなさん、こんにちは。"
	if !strings.Contains(got, want) {
		t.Fatalf("expected transcript to contain %q, got %q", want, got)
	}
}

func TestFormatSRTTimestampMS(t *testing.T) {
	if got, want := formatSRTTimestampMS(3723456), "01:02:03,456"; got != want {
		t.Fatalf("unexpected timestamp: got %q want %q", got, want)
	}
}
