package validations

type UserUpdate struct {
	ID                   uint   `form:"id" json:"id" validate:"required,is_exists_db=User ID"`
	CurrentPassword      string `form:"current_password" json:"current_password" validate:"omitempty,is_password_valid=User ID Password"`
	NewPassword          string `form:"new_password" json:"new_password" validate:"omitempty,required_with=CurrentPassword,min=1"`
	PasswordConfirmation string `form:"password_confirmation" json:"password_confirmation" validate:"eqfield=NewPassword"`
	Email                string `form:"email" json:"email" validate:"required,email,min=1,max=50"`
	Active               string `form:"active" json:"active" validate:"required,oneof=true false"`
	Admin                string `form:"admin" json:"admin" validate:"required,oneof=true false"`
}

type UserUpdateArbitrary struct {
	ID                   uint   `form:"id" json:"id" validate:"required,is_exists_db=User ID"`
	CurrentPassword      string `form:"current_password" json:"current_password" validate:"omitempty,is_password_valid=User ID Password"`
	NewPassword          string `form:"new_password" json:"new_password" validate:"omitempty,required_with=CurrentPassword,min=1"`
	PasswordConfirmation string `form:"password_confirmation" json:"password_confirmation" validate:"eqfield=NewPassword"`
	Email                string `form:"email" json:"email" validate:"omitempty,email,min=1,max=50"`
	Active               string `form:"active" json:"active" validate:"omitempty,oneof=true false"`
	Admin                string `form:"admin" json:"admin" validate:"omitempty,oneof=true false"`
}
