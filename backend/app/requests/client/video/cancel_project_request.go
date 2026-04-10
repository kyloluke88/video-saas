package video

type CancelProjectRequest struct {
	ProjectID string `json:"project_id" binding:"required,max=255"`
}
