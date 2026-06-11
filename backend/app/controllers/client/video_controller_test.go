package client

import (
	"regexp"
	"testing"

	"api/app/requests/client/video"
)

func TestBuildPodcastProjectIDUsesFixedTimestampPattern(t *testing.T) {
	projectID := buildPodcastProjectID("zh")
	pattern := regexp.MustCompile(`^zh_podcast_\d{14}$`)
	if !pattern.MatchString(projectID) {
		t.Fatalf("unexpected project_id format: %s", projectID)
	}
}

func TestResolvePodcastProjectIDKeepsOriginalProjectForRerun(t *testing.T) {
	projectID, err := resolvePodcastProjectID(video.CreatePodcastDialogueRequest{
		ProjectID: "zh_podcast_20260408154607",
	}, 1)
	if err != nil {
		t.Fatalf("resolvePodcastProjectID returned err: %v", err)
	}
	if projectID != "zh_podcast_20260408154607" {
		t.Fatalf("unexpected rerun project id: %s", projectID)
	}
}

func TestResolvePracticalProjectIDKeepsOriginalProjectForRerun(t *testing.T) {
	projectID, err := resolvePracticalProjectID(video.CreatePracticalDialogueRequest{
		ProjectID: "ja_practical_20260423162724",
	}, 1)
	if err != nil {
		t.Fatalf("resolvePracticalProjectID returned err: %v", err)
	}
	if projectID != "ja_practical_20260423162724" {
		t.Fatalf("unexpected rerun project id: %s", projectID)
	}
}

func TestBuildPodcastTaskPayloadUsesStageRange(t *testing.T) {
	payload := buildPodcastTaskPayload(map[string]interface{}{
		"project_id":       "zh_podcast_20260408154607",
		"run_mode":         1,
		"tts_type":         2,
		"start_from":       "render",
		"stop_at":          "persist",
		"lang":             "zh",
		"bg_img_filenames": []string{"bg-a.png"},
		"target_platform":  "tiktok",
		"aspect_ratio":     "9:16",
	})

	if got, _ := payload["start_from"].(string); got != "render" {
		t.Fatalf("unexpected start_from: %#v", payload["start_from"])
	}
	if got, _ := payload["stop_at"].(string); got != "persist" {
		t.Fatalf("unexpected stop_at: %#v", payload["stop_at"])
	}
	if _, exists := payload["source_project_id"]; exists {
		t.Fatalf("unexpected source_project_id in payload: %#v", payload["source_project_id"])
	}
	if _, exists := payload["specify_tasks"]; exists {
		t.Fatalf("unexpected specify_tasks in payload: %#v", payload["specify_tasks"])
	}
	if got, _ := payload["tts_type"].(int); got != 2 {
		t.Fatalf("unexpected tts_type: %#v", payload["tts_type"])
	}
	if got, _ := payload["target_platform"].(string); got != "tiktok" {
		t.Fatalf("unexpected target_platform: %#v", payload["target_platform"])
	}
	if got, _ := payload["aspect_ratio"].(string); got != "9:16" {
		t.Fatalf("unexpected aspect_ratio: %#v", payload["aspect_ratio"])
	}
}

func TestBuildPodcastTaskPayloadDefaultsGoogleToMultiple(t *testing.T) {
	payload := buildPodcastTaskPayload(map[string]interface{}{
		"project_id":       "zh_podcast_20260408154607",
		"run_mode":         0,
		"tts_type":         1,
		"lang":             "zh",
		"bg_img_filenames": []string{"bg-a.png"},
		"start_from":       "generate",
	})

	if got, _ := payload["is_multiple"].(int); got != 1 {
		t.Fatalf("unexpected is_multiple default: %#v", payload["is_multiple"])
	}
}

func TestBuildPodcastRequestPayloadStoresDefaultMetadata(t *testing.T) {
	payload := buildPodcastRequestPayload(video.CreatePodcastDialogueRequest{
		Lang:           "ja",
		ScriptFilename: "lesson.json",
	}, "ja_podcast_20260607010101", 0, nil, []string{"bg-a.png"}, 0)

	if got, _ := payload["start_from"].(string); got != "generate" {
		t.Fatalf("unexpected start_from: %#v", payload["start_from"])
	}
	if got, _ := payload["target_platform"].(string); got != "youtube" {
		t.Fatalf("unexpected target_platform: %#v", payload["target_platform"])
	}
	if got, _ := payload["aspect_ratio"].(string); got != "16:9" {
		t.Fatalf("unexpected aspect_ratio: %#v", payload["aspect_ratio"])
	}
	if got, _ := payload["resolution"].(string); got != defaultPodcastResolution() {
		t.Fatalf("unexpected resolution: %#v", payload["resolution"])
	}
	if got, _ := payload["design_style"].(int); got != 1 {
		t.Fatalf("unexpected design_style: %#v", payload["design_style"])
	}
	if got, _ := payload["tts_type"].(int); got != 1 {
		t.Fatalf("unexpected tts_type: %#v", payload["tts_type"])
	}
}

func TestBuildPracticalRequestPayloadStoresTTSType(t *testing.T) {
	payload := buildPracticalRequestPayload(video.CreatePracticalDialogueRequest{
		Lang:           "ja",
		TTSType:        2,
		ScriptFilename: "lesson.json",
	}, "ja_practical_20260607010101", 0, nil, nil)

	if got, _ := payload["tts_type"].(int); got != 2 {
		t.Fatalf("unexpected tts_type: %#v", payload["tts_type"])
	}
	if got, _ := payload["start_from"].(string); got != "generate" {
		t.Fatalf("unexpected start_from: %#v", payload["start_from"])
	}
}

func TestBuildPracticalTaskPayloadUsesStageRange(t *testing.T) {
	payload := buildPracticalTaskPayload(map[string]interface{}{
		"project_id":   "ja_practical_20260423162724",
		"run_mode":     1,
		"tts_type":     2,
		"start_from":   "images",
		"stop_at":      "render",
		"chapter_nums": []int{2, 6},
	})

	if got, _ := payload["start_from"].(string); got != "images" {
		t.Fatalf("unexpected start_from: %#v", payload["start_from"])
	}
	if got, _ := payload["stop_at"].(string); got != "render" {
		t.Fatalf("unexpected stop_at: %#v", payload["stop_at"])
	}
	if got, _ := payload["chapter_nums"].([]int); len(got) != 2 || got[0] != 2 || got[1] != 6 {
		t.Fatalf("unexpected chapter_nums: %#v", payload["chapter_nums"])
	}
	if _, exists := payload["source_project_id"]; exists {
		t.Fatalf("unexpected source_project_id in payload: %#v", payload["source_project_id"])
	}
	if _, exists := payload["specify_tasks"]; exists {
		t.Fatalf("unexpected specify_tasks in payload: %#v", payload["specify_tasks"])
	}
	if got, _ := payload["tts_type"].(int); got != 2 {
		t.Fatalf("unexpected tts_type: %#v", payload["tts_type"])
	}
}

func TestResolvePodcastStagePlanUsesStartStageTask(t *testing.T) {
	plan, err := resolvePodcastStagePlan(1, 1, "render", "persist")
	if err != nil {
		t.Fatalf("resolvePodcastStagePlan returned err: %v", err)
	}
	got, err := podcastTaskTypeForPlan(1, plan)
	if err != nil {
		t.Fatalf("podcastTaskTypeForPlan returned err: %v", err)
	}
	if got != "podcast.compose.render.v1" {
		t.Fatalf("unexpected task type: %s", got)
	}
}

func TestResolvePodcastStagePlanRejectsAlignForType2(t *testing.T) {
	if _, err := resolvePodcastStagePlan(2, 1, "align", "render"); err == nil {
		t.Fatalf("expected type2 align validation error")
	}
}

func TestResolvePracticalStagePlanUsesStartStageTask(t *testing.T) {
	plan, err := resolvePracticalStagePlan(1, 1, "render", "persist")
	if err != nil {
		t.Fatalf("resolvePracticalStagePlan returned err: %v", err)
	}
	got, err := practicalTaskTypeForPlan(1, plan)
	if err != nil {
		t.Fatalf("practicalTaskTypeForPlan returned err: %v", err)
	}
	if got != "practical.compose.render.v1" {
		t.Fatalf("unexpected task type: %s", got)
	}
}

func TestResolvePracticalStagePlanRejectsAlignForType2(t *testing.T) {
	if _, err := resolvePracticalStagePlan(2, 1, "align", "render"); err == nil {
		t.Fatalf("expected type2 align validation error")
	}
}
