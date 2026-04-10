package podcast

import dto "worker/services/podcast/model"

type ComposeInput struct {
	BackgroundImagePath string
	DialogueAudioPath   string
	Script              *dto.PodcastScript
	Resolution          string
	DesignStyle         int
	OutputPath          string
}
