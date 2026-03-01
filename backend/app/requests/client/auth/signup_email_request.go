package auth

type SignupEmailRequest struct {
	Email      string `json:"email" binding:"required,email,email_not_exists"`
	Password   string `json:"password" binding:"required"`
	RePassword string `json:"re_password" binding:"required,eqfield=Password"`
	FirstName  string `json:"first_name"`
	LastName   string `json:"last_name"`
}
