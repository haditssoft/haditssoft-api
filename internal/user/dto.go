package user

type UserResponse struct {
	ID        uint   `json:"id"`
	Email     string `json:"email"`
	Active    *bool  `json:"active"`
	Admin     *bool  `json:"admin"`
	CreatedBy string `json:"created_by"`
	UpdatedBy string `json:"updated_by"`
}

type CreateUserInput struct {
	Email                string `json:"email"`
	Password             string `json:"password"`
	PasswordConfirmation string `json:"password_confirmation"`
	Active               string `json:"active"`
	Admin                string `json:"admin"`
}

type UpdateUserInput struct {
	Email                string `json:"email"`
	Active               string `json:"active"`
	Admin                string `json:"admin"`
	NewPassword          string `json:"new_password"`
	CurrentPassword      string `json:"current_password"`
	PasswordConfirmation string `json:"password_confirmation"`
}

type VerifyInput struct {
	Code string `json:"code"`
}

type ForgotPasswordInput struct {
	Email string `json:"email"`
}

type ForgotPasswordConfirmInput struct {
	Email                string `json:"email"`
	Code                 string `json:"code"`
	NewPassword          string `json:"new_password"`
	PasswordConfirmation string `json:"password_confirmation"`
}

type GetListResponse struct {
	Data  interface{} `json:"data"`
	Total int64       `json:"total"`
	Page  int         `json:"page"`
	Limit int         `json:"limit"`
}

type AdminCreateUserInput struct {
	Email                string `json:"email"`
	Password             string `json:"password"`
	PasswordConfirmation string `json:"password_confirmation"`
	Active               string `json:"active"`
	Admin                string `json:"admin"`
}

type AdminUpdateUserInput struct {
	Email                string `json:"email"`
	Active               string `json:"active"`
	Admin                string `json:"admin"`
	NewPassword          string `json:"new_password"`
	PasswordConfirmation string `json:"password_confirmation"`
}
