package podcast

import "testing"

func TestBackgroundGraphForAlwaysStaticScale(t *testing.T) {
	graph := backgroundGraphFor("1080p")
	if graph != "[0:v]fps=30,scale=1920:1080,setsar=1,setpts=PTS-STARTPTS[bg]" {
		t.Fatalf("unexpected background graph: %s", graph)
	}
}
