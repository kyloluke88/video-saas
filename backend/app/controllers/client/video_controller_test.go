package client

import (
	"regexp"
	"testing"
)

func TestBuildPodcastProjectIDUsesFixedTimestampPattern(t *testing.T) {
	projectID := buildPodcastProjectID("zh")
	pattern := regexp.MustCompile(`^zh_podcast_\d{14}$`)
	if !pattern.MatchString(projectID) {
		t.Fatalf("unexpected project_id format: %s", projectID)
	}
}

func TestBuildPodcastReplayProjectIDUsesFixedReplayPattern(t *testing.T) {
	projectID := buildPodcastReplayProjectID("zh_podcast_20260408154607")
	pattern := regexp.MustCompile(`^zh_podcast_20260408154607__rm1__\d{14}$`)
	if !pattern.MatchString(projectID) {
		t.Fatalf("unexpected replay project_id format: %s", projectID)
	}
}

func TestBuildPodcastReplayProjectIDAlwaysUsesRootProjectID(t *testing.T) {
	projectID := buildPodcastReplayProjectID("zh_podcast_20260408154607__rm1__20260409171630")
	pattern := regexp.MustCompile(`^zh_podcast_20260408154607__rm1__\d{14}$`)
	if !pattern.MatchString(projectID) {
		t.Fatalf("unexpected replay project_id format: %s", projectID)
	}
}

func TestBuildPracticalReplayProjectIDAlwaysUsesRootProjectID(t *testing.T) {
	projectID := buildPracticalReplayProjectID("ja_practical_20260423162724__rm1__20260424000101")
	pattern := regexp.MustCompile(`^ja_practical_20260423162724__rm1__\d{14}$`)
	if !pattern.MatchString(projectID) {
		t.Fatalf("unexpected practical replay project_id format: %s", projectID)
	}
}

func TestBuildPodcastTaskPayloadIncludesSourceProjectIDForReplay(t *testing.T) {
	payload := buildPodcastTaskPayload(map[string]interface{}{
		"project_id":        "zh_podcast_20260408154607__rm1__20260409171630",
		"source_project_id": "zh_podcast_20260408154607__rm1__20260409171630",
		"run_mode":          1,
		"tts_type":          2,
		"specify_tasks":     []string{"generate", "render"},
		"lang":              "zh",
		"bg_img_filenames":  []string{"bg-a.png"},
	})

	if got, _ := payload["source_project_id"].(string); got != "zh_podcast_20260408154607__rm1__20260409171630" {
		t.Fatalf("unexpected source_project_id: %#v", payload["source_project_id"])
	}
	if got, _ := payload["specify_tasks"].([]string); len(got) != 2 || got[0] != "generate" {
		t.Fatalf("unexpected specify_tasks: %#v", payload["specify_tasks"])
	}
	if got, _ := payload["tts_type"].(int); got != 2 {
		t.Fatalf("unexpected tts_type: %#v", payload["tts_type"])
	}
}

func TestBuildPodcastTaskPayloadDefaultsGoogleToMultiple(t *testing.T) {
	payload := buildPodcastTaskPayload(map[string]interface{}{
		"project_id":       "zh_podcast_20260408154607",
		"run_mode":         0,
		"tts_type":         1,
		"lang":             "zh",
		"bg_img_filenames": []string{"bg-a.png"},
	})

	if got, _ := payload["is_multiple"].(int); got != 1 {
		t.Fatalf("unexpected is_multiple default: %#v", payload["is_multiple"])
	}
}

func TestBuildPracticalTaskPayloadIncludesSourceProjectIDForReplay(t *testing.T) {
	payload := buildPracticalTaskPayload(map[string]interface{}{
		"project_id":             "ja_practical_20260423162724__rm1__20260424001010",
		"source_project_id":      "ja_practical_20260423162724",
		"run_mode":               1,
		"specify_tasks":          []string{"render", "persist"},
		"block_bg_img_filenames": []string{"block1.png", "block2.png"},
	})

	if got, _ := payload["source_project_id"].(string); got != "ja_practical_20260423162724" {
		t.Fatalf("unexpected source_project_id: %#v", payload["source_project_id"])
	}
	if got, _ := payload["specify_tasks"].([]string); len(got) != 2 || got[0] != "render" || got[1] != "persist" {
		t.Fatalf("unexpected specify_tasks: %#v", payload["specify_tasks"])
	}
	if got, _ := payload["tts_type"].(int); got != 1 {
		t.Fatalf("unexpected tts_type: %#v", payload["tts_type"])
	}
	if got, _ := payload["block_bg_img_filenames"].([]string); len(got) != 2 || got[0] != "block1.png" {
		t.Fatalf("unexpected block_bg_img_filenames: %#v", payload["block_bg_img_filenames"])
	}
}

func TestBuildPodcastTaskPayloadPassesIsMultipleFlag(t *testing.T) {
	payload := buildPodcastTaskPayload(map[string]interface{}{
		"project_id":       "zh_podcast_20260408154607",
		"run_mode":         0,
		"tts_type":         1,
		"lang":             "zh",
		"is_multiple":      0,
		"bg_img_filenames": []string{"bg-a.png"},
	})
	if got, _ := payload["is_multiple"].(int); got != 0 {
		t.Fatalf("unexpected is_multiple=0 payload: %#v", payload["is_multiple"])
	}

	payload = buildPodcastTaskPayload(map[string]interface{}{
		"project_id":       "zh_podcast_20260408154607",
		"run_mode":         0,
		"tts_type":         1,
		"lang":             "zh",
		"is_multiple":      1,
		"bg_img_filenames": []string{"bg-a.png"},
	})
	if got, _ := payload["is_multiple"].(int); got != 1 {
		t.Fatalf("unexpected is_multiple=1 payload: %#v", payload["is_multiple"])
	}
}

func TestPodcastTaskTypeForInitialStageUsesStageEntryTask(t *testing.T) {
	cases := map[int]string{
		0: "podcast.audio.generate.v1",
		1: "podcast.audio.generate.v1",
	}

	for runMode, want := range cases {
		got, err := podcastTaskTypeForInitialStage(1, runMode, []string{"generate"})
		if err != nil {
			t.Fatalf("podcastTaskTypeForInitialStage returned err: %v", err)
		}
		if got != want {
			t.Fatalf("run_mode=%d task type mismatch: got=%s want=%s", runMode, got, want)
		}
	}

	type2Render, err := podcastTaskTypeForInitialStage(2, 1, []string{"finalize", "render"})
	if err != nil {
		t.Fatalf("podcastTaskTypeForInitialStage returned err: %v", err)
	}
	if type2Render != "podcast.compose.render.v1" {
		t.Fatalf("unexpected type2 replay entry: %s", type2Render)
	}
}

func TestNormalizePodcastSpecifyTasksOrdersByPipeline(t *testing.T) {
	tasks, err := normalizePodcastSpecifyTasks(1, []string{"persist", "generate", "render"})
	if err != nil {
		t.Fatalf("normalizePodcastSpecifyTasks returned err: %v", err)
	}
	if len(tasks) != 3 || tasks[0] != "generate" || tasks[1] != "render" || tasks[2] != "persist" {
		t.Fatalf("unexpected normalized tasks: %#v", tasks)
	}
}

func TestNormalizePodcastSpecifyTasksRejectsAlignForType2(t *testing.T) {
	if _, err := normalizePodcastSpecifyTasks(2, []string{"generate", "align"}); err == nil {
		t.Fatalf("expected type2 align validation error")
	}
}

func TestPracticalTaskTypeForInitialStageUsesStageEntryTask(t *testing.T) {
	cases := map[int]string{
		0: "practical.audio.generate.v1",
		1: "practical.compose.render.v1",
	}

	for runMode, want := range cases {
		specifyTasks := []string(nil)
		if runMode == 1 {
			specifyTasks = []string{"render", "persist"}
		}
		got, err := practicalTaskTypeForInitialStage(runMode, specifyTasks)
		if err != nil {
			t.Fatalf("practicalTaskTypeForInitialStage returned err: %v", err)
		}
		if got != want {
			t.Fatalf("run_mode=%d task type mismatch: got=%s want=%s", runMode, got, want)
		}
	}
}

func TestNormalizePracticalSpecifyTasksOrdersByPipeline(t *testing.T) {
	tasks, err := normalizePracticalSpecifyTasks([]string{"persist", "render", "finalize"})
	if err != nil {
		t.Fatalf("normalizePracticalSpecifyTasks returned err: %v", err)
	}
	if len(tasks) != 3 || tasks[0] != "render" || tasks[1] != "finalize" || tasks[2] != "persist" {
		t.Fatalf("unexpected normalized tasks: %#v", tasks)
	}
}
