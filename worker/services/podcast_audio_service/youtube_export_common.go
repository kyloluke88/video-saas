package podcast_audio_service

import (
	"path/filepath"
	"strings"

	conf "worker/pkg/config"
	ffmpegcommon "worker/services/ffmpeg_service/common"
)

func youtubePublishLeadInMS(language string) int {
	introPath := youtubeIntroAnimationPath(language)
	if strings.TrimSpace(introPath) == "" {
		return 0
	}
	durationSec, err := ffmpegcommon.AudioDurationSec(introPath)
	if err != nil || durationSec <= 0 {
		return 0
	}
	return int(durationSec * 1000)
}

func youtubeIntroAnimationPath(language string) string {
	language = strings.TrimSpace(strings.ToLower(language))
	if language != "zh" && language != "ja" {
		return ""
	}
	candidates := []string{
		filepath.Join(conf.Get[string]("worker.worker_assets_dir"), "podcast", "animation", language+"_open.mp4"),
		filepath.Join("assets", "podcast", "animation", language+"_open.mp4"),
		filepath.Join("worker", "assets", "podcast", "animation", language+"_open.mp4"),
		filepath.Join("/Users/luca/go/github.com/luca/video-saas/worker/assets/podcast/animation", language+"_open.mp4"),
	}
	for _, candidate := range candidates {
		if fileExists(candidate) {
			return candidate
		}
	}
	return ""
}
