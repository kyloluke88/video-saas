package auth

type SigninByPasswordRequest struct {
	LoginId  string `json:"login_id" binding:"required"`
	Password string `json:"password" binding:"required"`
}
