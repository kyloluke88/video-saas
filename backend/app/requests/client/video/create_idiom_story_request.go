package video

type CreateIdiomStoryRequest struct {
	IdiomName         string `json:"idiom_name" binding:"required,max=100"`
	IdiomNameEn       string `json:"idiom_name_en" binding:"required,max=100"`
	Platform          string `json:"platform" binding:"omitempty,oneof=tiktok youtube both"`
	NarrationLanguage string `json:"narration_language" binding:"omitempty,max=20"`
	TargetDurationSec int    `json:"target_duration_sec" binding:"omitempty,min=15,max=180"`
	AspectRatio       string `json:"aspect_ratio" binding:"omitempty,oneof=1:1 16:9 9:16 4:3 3:4 21:9 9:21"`
	Resolution        string `json:"resolution" binding:"omitempty,oneof=480p 720p"`
	Tone              string `json:"tone" binding:"omitempty,max=100"`
}
