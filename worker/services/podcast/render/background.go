package podcast

import (
	"fmt"
	ffmpegcommon "worker/services/media/ffmpeg/common"
)

func backgroundGraphFor(resolution string) string {
	return fmt.Sprintf("[0:v]scale=%s[bg]", ffmpegcommon.ResolutionToScale(resolution))
}
