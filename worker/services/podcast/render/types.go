package podcast

import dto "worker/services/podcast/model"

type ComposeInput struct {
	BackgroundImagePath string
	DialogueAudioPath   string
	Script              *dto.PodcastScript
	Language            string
	Resolution          string
	DesignStyle         int
	OutputPath          string
}
