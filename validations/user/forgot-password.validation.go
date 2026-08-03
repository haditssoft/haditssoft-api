package validations

type ForgotPasswordRequest struct {
	Email string `form:"email" json:"email" validate:"required,email,min=1,max=50"`
}

type ForgotPasswordConfirm struct {
	Email                string `form:"email" json:"email" validate:"required,email,min=1,max=50"`
	Code                 string `form:"code" json:"code" validate:"required,len=6,numeric"`
	NewPassword          string `form:"new_password" json:"new_password" validate:"required,min=1"`
	PasswordConfirmation string `form:"password_confirmation" json:"password_confirmation" validate:"eqfield=NewPassword"`
}
