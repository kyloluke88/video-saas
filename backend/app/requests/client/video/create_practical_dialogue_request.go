package video

type CreatePracticalDialogueRequest struct {
	ProjectID           string   `json:"project_id" binding:"omitempty,max=255"`
	Lang                string   `json:"lang" binding:"omitempty,oneof=zh ja"`
	RunMode             int      `json:"run_mode" binding:"omitempty,oneof=0 1"`
	SpecifyTasks        []string `json:"specify_tasks" binding:"omitempty,dive,oneof=generate align render finalize persist"`
	BlockNums           []int    `json:"block_nums" binding:"omitempty,dive,min=1"`
	ScriptFilename      string   `json:"script_filename" binding:"omitempty,max=255"`
	BgImgFilenames      []string `json:"bg_img_filenames" binding:"omitempty,dive,max=255"`
	BlockBgImgFilenames []string `json:"block_bg_img_filenames" binding:"omitempty,dive,max=255"`
	Resolution          string   `json:"resolution" binding:"omitempty,oneof=480p 720p 1080p 1440p 2000p"`
	DesignType          int      `json:"design_type" binding:"omitempty,oneof=1 2"`
}
