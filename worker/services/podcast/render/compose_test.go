package podcast

import (
	"path/filepath"
	"testing"
)

func TestPodcastDesignType1AnimationPath(t *testing.T) {
	if got := podcastDesignAnimationPath("en"); got != "" {
		t.Fatalf("expected no animation path for unsupported language, got %q", got)
	}
	if got := podcastDesignAnimationPath("ja"); got == "" || filepath.Base(got) != "headphone.gif" {
		t.Fatalf("expected ja animation path, got %q", got)
	}
	if got := podcastDesignAnimationPath("zh"); got == "" || filepath.Base(got) != "headphone.gif" {
		t.Fatalf("expected zh animation path, got %q", got)
	}
}

func TestPodcastDesignType1AnimationFilter(t *testing.T) {
	got := podcastDesignType1AnimationFilter("480p")
	want := "[2:v]fps=15,scale=61:61:flags=lanczos,format=rgba[anim];[v0][anim]overlay=15:14:shortest=1:eof_action=pass"
	if got != want {
		t.Fatalf("unexpected 480p filter\nwant: %s\ngot:  %s", want, got)
	}

	got = podcastDesignType1AnimationFilter("1080p")
	want = "[2:v]fps=15,scale=88:88:flags=lanczos,format=rgba[anim];[v0][anim]overlay=34:28:shortest=1:eof_action=pass"
	if got != want {
		t.Fatalf("unexpected 1080p filter\nwant: %s\ngot:  %s", want, got)
	}
}

func TestPodcastDesignType1LogoPath(t *testing.T) {
	if got := podcastDesignLogoPath("ja"); got == "" || filepath.Base(got) != "ja_logo.png" {
		t.Fatalf("expected ja logo path, got %q", got)
	}
	if got := podcastDesignLogoPath("zh"); got != "" {
		t.Fatalf("expected no zh logo path, got %q", got)
	}
}

func TestAppendLoopedImageInput(t *testing.T) {
	got := appendLoopedImageInput([]string{"-y"}, "ja_logo.png")
	want := []string{"-y", "-loop", "1", "-i", "ja_logo.png"}
	if len(got) != len(want) {
		t.Fatalf("unexpected arg length: got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("unexpected args: got %v want %v", got, want)
		}
	}
}
