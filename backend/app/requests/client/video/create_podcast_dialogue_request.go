package video

type CreatePodcastDialogueRequest struct {
	ProjectID      string   `json:"project_id" binding:"omitempty,max=255"`
	Title          string   `json:"title" binding:"omitempty,max=200"`
	Lang           string   `json:"lang" binding:"omitempty,oneof=zh ja"`
	ContentProfile string   `json:"content_profile" binding:"omitempty,oneof=daily social_issue international"`
	TTSType        int      `json:"tts_type" binding:"omitempty,oneof=1 2"`
	RunMode        int      `json:"run_mode" binding:"omitempty,oneof=0 1 2 3 4"`
	BlockNums      []int    `json:"block_nums" binding:"omitempty,dive,min=1"`
	BlockNum       []int    `json:"block_num" binding:"omitempty,dive,min=1"`
	ScriptFilename string   `json:"script_filename" binding:"omitempty,max=255"`
	BgImgFilenames []string `json:"bg_img_filenames" binding:"omitempty,dive,max=255"`
	TargetPlatform string   `json:"target_platform" binding:"omitempty,oneof=youtube tiktok"`
	AspectRatio    string   `json:"aspect_ratio" binding:"omitempty,oneof=1:1 16:9 9:16 4:3 3:4 21:9 9:21"`
	Resolution     string   `json:"resolution" binding:"omitempty,oneof=480p 720p 1080p 1440p 2000p"`
	DesignStyle    int      `json:"design_style" binding:"omitempty,oneof=1 2"`
}
