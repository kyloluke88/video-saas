package podcast

import conf "worker/pkg/config"

func podcastHighlightEnabled() bool {
	return conf.Get[bool]("worker.mfa_enabled")
}

func podcastVideoFPS() int {
	value := conf.Get[int]("worker.podcast_fps")
	if value <= 0 {
		return 30
	}
	return value
}
