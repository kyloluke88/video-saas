package podcast

import conf "worker/pkg/config"

func podcastHighlightEnabled() bool {
	return conf.Get[bool]("worker.mfa_enabled")
}
