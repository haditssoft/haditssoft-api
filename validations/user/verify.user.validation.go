package validations

type UserVerify struct {
	Code string `form:"code" json:"code" validate:"required,len=6,numeric"`
}
