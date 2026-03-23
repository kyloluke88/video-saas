package podcast_audio_service

import (
	"strings"
	"testing"

	"worker/internal/dto"
)

func TestBuildYouTubePublishTextWithLeadIn_JapaneseUsesUnifiedFormat(t *testing.T) {
	script := dto.PodcastScript{
		Language: "ja",
		YouTube: dto.PodcastYouTube{
			PublishTitle: "Oshikatsu Culture for Everyday Japanese Learners | 若い人に広がる推活文化をゆるく話そう",
			Chapters: []dto.PodcastYouTubeChapter{
				{ChapterID: "ch_001", TitleEN: "Opening", BlockIDs: []string{"block_001"}},
				{ChapterID: "ch_002", TitleEN: "Why It Matters", BlockIDs: []string{"block_002"}},
			},
			InThisEpisodeYouWillLearn: []string{
				"What oshikatsu means in modern Japanese culture.",
			},
			DescriptionIntro: []string{
				"This episode gives you slow and natural Japanese listening practice.",
			},
		},
		Blocks: []dto.PodcastBlock{
			{
				ChapterID: "ch_001",
				BlockID:   "block_001",
				Segments: []dto.PodcastSegment{
					{SegmentID: "seg_001", StartMS: 0, EndMS: 5000, Text: "こんにちは。"},
				},
			},
			{
				ChapterID: "ch_002",
				BlockID:   "block_002",
				Segments: []dto.PodcastSegment{
					{SegmentID: "seg_002", StartMS: 60000, EndMS: 65000, Text: "推し活について話しましょう。"},
				},
			},
		},
	}

	got := buildYouTubePublishTextWithLeadIn(script, 4070)
	for _, want := range []string{
		"Title:",
		"Oshikatsu Culture for Everyday Japanese Learners | 若い人に広がる推活文化をゆるく話そう | Japanese Daily Podcast",
		"Hashtags:",
		"0:00 Opening",
		"1:04 Why It Matters",
		"In this episode, you will learn:",
		"Studio Tags (paste into YouTube Tags field only, comma-separated phrases are OK):",
		"learn japanese",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected publish text to contain %q, got %q", want, got)
		}
	}
	for _, unwanted := range []string{
		"Description:",
		"mandarin podcast",
		"study chinese",
		"learn mandarin",
		"中文",
		"汉语",
	} {
		if strings.Contains(got, unwanted) {
			t.Fatalf("expected publish text to omit %q, got %q", unwanted, got)
		}
	}
}

func TestBuildStudioTags_FiltersCrossLanguageTags(t *testing.T) {
	zhScript := dto.PodcastScript{
		Language: "zh",
		YouTube: dto.PodcastYouTube{
			VideoTags: []string{"learn chinese", "日本語", "japanese listening", "中文听力"},
		},
	}
	zhTags := buildStudioTags(zhScript)
	zhJoined := strings.Join(zhTags, ",")
	if strings.Contains(zhJoined, "japanese listening") || strings.Contains(zhJoined, "日本語") {
		t.Fatalf("expected zh studio tags to remove japanese tags, got %q", zhJoined)
	}

	jaScript := dto.PodcastScript{
		Language: "ja",
		YouTube: dto.PodcastYouTube{
			VideoTags: []string{"learn japanese", "study chinese", "中文", "日本語"},
		},
	}
	jaTags := buildStudioTags(jaScript)
	jaJoined := strings.Join(jaTags, ",")
	if strings.Contains(jaJoined, "study chinese") || strings.Contains(jaJoined, "中文") {
		t.Fatalf("expected ja studio tags to remove chinese tags, got %q", jaJoined)
	}
}
