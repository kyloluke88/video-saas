package client

type UpdateUserInfoRequest struct {
	Email *string `json:"email" binding:"required,email,email_not_exists"` // todo 关于指针这里的需要测试
}
