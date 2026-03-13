package podcast

import "worker/internal/dto"

type ComposeInput struct {
	BackgroundImagePath string
	DialogueAudioPath   string
	Script              *dto.PodcastScript
	Resolution          string
	DesignStyle         int
	OutputPath          string
}
