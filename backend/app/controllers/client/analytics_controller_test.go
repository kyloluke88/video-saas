package client

import (
	"testing"

	analyticsModel "api/app/models/analytics"
)

func TestBuildPageViewDedupKey(t *testing.T) {
	// fingerprint 必须稳定，否则 Redis 去重会随机失效。
	event := analyticsModel.PageViewEvent{
		VisitorKey: "123e4567-e89b-12d3-a456-426614174000",
		SessionKey: "123e4567-e89b-12d3-a456-426614174001",
		PageType:   int16(analyticsModel.PageTypePodcastScriptDetail),
		PagePath:   "/podcast/scripts/demo",
	}

	keyA := buildPageViewDedupKey(event)
	keyB := buildPageViewDedupKey(event)
	if keyA != keyB {
		t.Fatalf("expected identical keys, got %q and %q", keyA, keyB)
	}

	event.PagePath = "/podcast/scripts/other"
	keyC := buildPageViewDedupKey(event)
	if keyA == keyC {
		t.Fatal("expected different paths to produce different keys")
	}
}
