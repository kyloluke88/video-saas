package podcast

import (
	"strings"

	dto "worker/services/podcast/model"
)

func WritePodcastASS(script dto.PodcastScript, projectDir, resolution string, style int) (string, error) {
	switch strings.ToLower(strings.TrimSpace(script.Language)) {
	case "ja", "ja-jp":
		return writeJapaneseASS(script, projectDir, resolution, style)
	default:
		return writeChineseASS(script, projectDir, resolution, style)
	}
}
