package workerapp

import "testing"

func TestSchedulerForMainRoleExcludesAlignOnlyTasks(t *testing.T) {
	scheduler := SchedulerForRole("main")

	if _, ok := scheduler["podcast.compose.render.v1"]; !ok {
		t.Fatal("main scheduler should include podcast render tasks")
	}
	if _, ok := scheduler["podcast.compose.finalize.v1"]; !ok {
		t.Fatal("main scheduler should include podcast finalize tasks")
	}
	if _, ok := scheduler["podcast.audio.align.v1"]; ok {
		t.Fatal("main scheduler should not include podcast align tasks")
	}
	if _, ok := scheduler["practical.audio.align.v1"]; ok {
		t.Fatal("main scheduler should not include practical align tasks")
	}
}

func TestSchedulerForAlignRoleIncludesOnlyAlignTasks(t *testing.T) {
	scheduler := SchedulerForRole("align")

	if _, ok := scheduler["podcast.audio.align.v1"]; !ok {
		t.Fatal("align scheduler should include podcast align tasks")
	}
	if _, ok := scheduler["practical.audio.align.v1"]; !ok {
		t.Fatal("align scheduler should include practical align tasks")
	}
	if _, ok := scheduler["podcast.compose.render.v1"]; ok {
		t.Fatal("align scheduler should not include podcast render tasks")
	}
	if _, ok := scheduler["upload.v1"]; ok {
		t.Fatal("align scheduler should not include upload tasks")
	}
}
