package video

type CreatePracticalDialogueRequest struct {
	ProjectID      string `json:"project_id" binding:"omitempty,max=255"`
	Lang           string `json:"lang" binding:"omitempty,oneof=zh ja"`
	TTSType        int    `json:"tts_type" binding:"omitempty,oneof=1 2"`
	RunMode        int    `json:"run_mode" binding:"omitempty,oneof=0 1"`
	StartFrom      string `json:"start_from" binding:"omitempty,oneof=generate align images render persist"`
	StopAt         string `json:"stop_at" binding:"omitempty,oneof=generate align images render persist"`
	BlockNums      []int  `json:"block_nums" binding:"omitempty,dive,min=1"`
	ChapterNums    []int  `json:"chapter_nums" binding:"omitempty,dive,min=1"`
	ScriptFilename string `json:"script_filename" binding:"omitempty,max=255"`
	Resolution     string `json:"resolution" binding:"omitempty,oneof=480p 720p 1080p 1440p 2000p"`
	DesignType     int    `json:"design_type" binding:"omitempty,oneof=1 2"`
}
