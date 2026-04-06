package podcast_page_service

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildPageUpsertFromProjectDir(t *testing.T) {
	projectID := "zh_podcast_20260401165006_json"
	projectDir := filepath.Clean(filepath.Join("..", "..", "outputs", "projects", projectID))

	upsert, err := BuildPageUpsertFromProjectDir(projectDir, PersistInput{
		ProjectID: projectID,
		VideoURL:  "https://cdn.example.com/projects/zh_podcast_20260401165006_json/final.mp4",
	})
	if err != nil {
		t.Fatalf("BuildPageUpsertFromProjectDir failed: %v", err)
	}

	if upsert.ProjectID != projectID {
		t.Fatalf("unexpected project_id: %s", upsert.ProjectID)
	}
	if upsert.Title == "" || !strings.Contains(upsert.Title, "外国人第一次来中国") {
		t.Fatalf("unexpected title: %s", upsert.Title)
	}
	if upsert.VideoURL == "" || !strings.Contains(upsert.VideoURL, "cdn.example.com") {
		t.Fatalf("unexpected video_url: %s", upsert.VideoURL)
	}
	if len(upsert.Script) == 0 || !strings.Contains(string(upsert.Script), "\"sections\"") {
		t.Fatalf("script json was not built correctly: %s", string(upsert.Script))
	}
	if upsert.Slug == "" || !strings.Contains(upsert.Slug, "what-panics-first-timers-in-china") {
		t.Fatalf("unexpected slug: %s", upsert.Slug)
	}
}
