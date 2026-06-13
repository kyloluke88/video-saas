package podcast

import (
	"fmt"
	ffmpegcommon "worker/services/media/ffmpeg/common"
)

func backgroundGraphFor(resolution string) string {
	return fmt.Sprintf(
		"[0:v]%s[bg]",
		podcastVideoFilterChain(fmt.Sprintf("scale=%s", ffmpegcommon.ResolutionToScale(resolution))),
	)
}
