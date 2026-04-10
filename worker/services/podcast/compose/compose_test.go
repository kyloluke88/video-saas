package podcast_compose_service

import "testing"

func TestBackgroundImagePathsForUsesOnlyFirstBackground(t *testing.T) {
	path, err := backgroundImagePathForRequest([]string{"a.png", "b.png", "c.png"})
	if err != nil {
		t.Fatalf("backgroundImagePathForRequest returned err: %v", err)
	}
	if path == "" {
		t.Fatalf("expected non-empty background path")
	}
}

func TestBackgroundImagePathsForRequiresBackgrounds(t *testing.T) {
	if _, err := backgroundImagePathForRequest(nil); err == nil {
		t.Fatalf("expected bg_img_filenames required error")
	}
}
