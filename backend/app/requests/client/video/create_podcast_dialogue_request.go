package video

type CreatePodcastDialogueRequest struct {
	Title          string `json:"title" binding:"omitempty,max=200"`
	Lang           string `json:"lang" binding:"required,oneof=zh ja"`
	ContentProfile string `json:"content_profile" binding:"required,oneof=daily social_issue international"`
	IsDirect       int    `json:"is_direct" binding:"omitempty,oneof=0 1"`
	ScriptFilename string `json:"script_filename" binding:"required,max=255"`
	BgImgFilename  string `json:"bg_img_filename" binding:"required,max=255"`
	TargetPlatform string `json:"target_platform" binding:"omitempty,oneof=youtube tiktok"`
	AspectRatio    string `json:"aspect_ratio" binding:"omitempty,oneof=1:1 16:9 9:16 4:3 3:4 21:9 9:21"`
	Resolution     string `json:"resolution" binding:"omitempty,oneof=480p 720p 1080p 1440p 2000p"`
	DesignStyle    int    `json:"design_style" binding:"omitempty,min=1,max=3"`
}
