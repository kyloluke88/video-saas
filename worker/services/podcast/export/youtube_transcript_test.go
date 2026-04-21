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
				EN:        "Hello everyone, welcome to our daily Chinese channel.",
				StartMS:   0,
				EndMS:     2450,
			},
			{
				SegmentID: "seg_002",
				Text:      "今天我们来聊一聊年轻人的婚恋观。",
				EN:        "Today we are going to talk about young people's views on marriage and relationships.",
				StartMS:   2450,
				EndMS:     5870,
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
				EN:        "Hello everyone, welcome to our daily Chinese channel.",
				StartMS:   0,
				EndMS:     2450,
			},
			{
				SegmentID: "seg_002",
				Text:      "今天我们来聊一聊年轻人的婚恋观。",
				EN:        "Today we are going to talk about young people's views on marriage and relationships.",
				StartMS:   2450,
				EndMS:     5870,
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
