package client

import (
	"regexp"
	"testing"

	video "api/app/requests/client/video"
)

func TestBuildPodcastProjectIDUsesFixedTimestampPattern(t *testing.T) {
	projectID := buildPodcastProjectID("zh")
	pattern := regexp.MustCompile(`^zh_podcast_\d{14}$`)
	if !pattern.MatchString(projectID) {
		t.Fatalf("unexpected project_id format: %s", projectID)
	}
}

func TestBuildPodcastReplayProjectIDUsesFixedReplayPattern(t *testing.T) {
	projectID := buildPodcastReplayProjectID("zh_podcast_20260408154607", 2)
	pattern := regexp.MustCompile(`^zh_podcast_20260408154607__rm2__\d{14}$`)
	if !pattern.MatchString(projectID) {
		t.Fatalf("unexpected replay project_id format: %s", projectID)
	}
}

func TestBuildPodcastReplayProjectIDAlwaysUsesRootProjectID(t *testing.T) {
	projectID := buildPodcastReplayProjectID("zh_podcast_20260408154607__rm1__20260409171630", 3)
	pattern := regexp.MustCompile(`^zh_podcast_20260408154607__rm3__\d{14}$`)
	if !pattern.MatchString(projectID) {
		t.Fatalf("unexpected replay project_id format: %s", projectID)
	}
}

func TestBuildPodcastTaskPayloadIncludesSourceProjectIDForReplay(t *testing.T) {
	payload := buildPodcastTaskPayload(video.CreatePodcastDialogueRequest{
		ProjectID: "zh_podcast_20260408154607__rm1__20260409171630",
		Lang:      "zh",
	}, "zh_podcast_20260408154607__rm3__20260409180433", 3, nil, nil, 0)

	if got, _ := payload["source_project_id"].(string); got != "zh_podcast_20260408154607__rm1__20260409171630" {
		t.Fatalf("unexpected source_project_id: %#v", payload["source_project_id"])
	}
}
