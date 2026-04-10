package podcast_export_service

import (
	"strings"
	"testing"

	dto "worker/services/podcast/model"
)

func TestBuildYouTubePublishTextWithLeadIn_JapaneseUsesUnifiedFormat(t *testing.T) {
	pageURL := "https://podcast.lucayo.com/podcast/scripts/oshikatsu-culture-for-everyday-japanese-learners"
	script := dto.PodcastScript{
		Language: "ja",
		EnTitle:  "Oshikatsu Culture for Everyday Japanese Learners",
		Vocabulary: []byte(`[
			{"term":"推し活","meaning":"fan activities to support your favorite idol or creator"}
		]`),
		Grammar: []byte(`[
			{"pattern":"〜について","meaning":"about; regarding"}
		]`),
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

	got := buildYouTubePublishTextWithLeadIn(script, pageURL, 4070)
	for _, want := range []string{
		"This episode gives you slow and natural Japanese listening practice.",
		"Title:",
		"Oshikatsu Culture for Everyday Japanese Learners | 若い人に広がる推活文化をゆるく話そう | Japanese Daily Podcast",
		"Hashtags:",
		"00:00 Opening",
		"01:04 Why It Matters",
		"Key vocabulary and grammar from this episode:",
		"Vocabulary:",
		"- 推し活: fan activities to support your favorite idol or creator",
		"Grammar:",
		"- 〜について: about; regarding",
		"Read the full podcast script and download the PDF study sheet here:",
		pageURL,
		"In this episode, you will learn:",
		"Studio Tags (paste into YouTube Tags field only, comma-separated phrases are OK):",
		"learn japanese",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected publish text to contain %q, got %q", want, got)
		}
	}
	if !strings.HasPrefix(got, "This episode gives you slow and natural Japanese listening practice.") {
		t.Fatalf("expected description intro at top, got %q", got)
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

func TestBuildYouTubePublishTextWithLeadIn_PlacesStudyHighlightsAfterChapters(t *testing.T) {
	pageURL := "https://podcast.lucayo.com/podcast/scripts/why-chinese-delivery-is-so-fast"
	script := dto.PodcastScript{
		Language: "zh",
		EnTitle:  "Why Chinese Delivery Is So Fast",
		Vocabulary: []byte(`[
			{"term":"外卖","meaning":"food delivery"}
		]`),
		Grammar: []byte(`[
			{"pattern":"一...就...","meaning":"as soon as... then..."}
		]`),
		YouTube: dto.PodcastYouTube{
			PublishTitle: "Why Chinese Delivery Is So Fast | 为什么中国的外卖这么快？",
			Chapters: []dto.PodcastYouTubeChapter{
				{ChapterID: "ch_001", TitleEN: "Opening", BlockIDs: []string{"block_001"}},
			},
		},
		Blocks: []dto.PodcastBlock{
			{
				ChapterID: "ch_001",
				BlockID:   "block_001",
				Segments: []dto.PodcastSegment{
					{SegmentID: "seg_001", StartMS: 0, EndMS: 5000, Text: "大家好。"},
				},
			},
		},
	}

	got := buildYouTubePublishTextWithLeadIn(script, pageURL, 0)
	chapterIndex := strings.Index(got, "00:00 Opening")
	highlightIndex := strings.Index(got, "Key vocabulary and grammar from this episode:")
	if chapterIndex == -1 || highlightIndex == -1 {
		t.Fatalf("expected publish text to contain chapters and highlights, got %q", got)
	}
	if highlightIndex <= chapterIndex {
		t.Fatalf("expected highlights after chapters, got %q", got)
	}
	ctaIndex := strings.Index(got, pageURL)
	if ctaIndex == -1 {
		t.Fatalf("expected publish text to contain script page url, got %q", got)
	}
	if ctaIndex <= highlightIndex {
		t.Fatalf("expected CTA after highlights, got %q", got)
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
