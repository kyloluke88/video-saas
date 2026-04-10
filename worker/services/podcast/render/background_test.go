package podcast

import "testing"

func TestBackgroundGraphForAlwaysStaticScale(t *testing.T) {
	graph := backgroundGraphFor("1080p")
	if graph != "[0:v]scale=1920:1080[bg]" {
		t.Fatalf("unexpected background graph: %s", graph)
	}
}
