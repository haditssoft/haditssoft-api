package validations

type UserCreate struct {
	ID                   uint   `form:"id" json:"id"`
	Password             string `form:"password" json:"password" validate:"required,min=1"`
	PasswordConfirmation string `form:"password_confirmation" json:"password_confirmation" validate:"eqfield=Password"`
	Email                string `form:"email" json:"email" validate:"required,email,min=1,max=50"`
	Active               string `form:"active" json:"active" validate:"required,oneof=true false"`
	Admin                string `form:"admin" json:"admin" validate:"required,oneof=true false"`
}
