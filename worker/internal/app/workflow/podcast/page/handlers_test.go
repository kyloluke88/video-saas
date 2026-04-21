package podcast_page

import "testing"

func TestBuildReplayRequestPayloadPatchIncludesRunModeAndSourceProjectID(t *testing.T) {
	patch := buildReplayRequestPayloadPatch(persistPayload{
		ProjectID:       "ja_podcast_20260419123701__rm1__20260420164553",
		SourceProjectID: "ja_podcast_20260419123701",
		RunMode:         1,
		SpecifyTasks:    []string{"persist"},
	})

	if got, ok := patch["run_mode"].(int); !ok || got != 1 {
		t.Fatalf("unexpected run_mode patch: %#v", patch["run_mode"])
	}
	if got, ok := patch["source_project_id"].(string); !ok || got != "ja_podcast_20260419123701" {
		t.Fatalf("unexpected source_project_id patch: %#v", patch["source_project_id"])
	}
	if got, ok := patch["specify_tasks"].([]string); !ok || len(got) != 1 || got[0] != "persist" {
		t.Fatalf("unexpected specify_tasks patch: %#v", patch["specify_tasks"])
	}
}
