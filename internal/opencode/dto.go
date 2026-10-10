package opencode

type AskOpenCodeRequest struct {
	Kitab    string `form:"kitab" json:"kitab" validate:"required"`
	Nomer    string `form:"nomer" json:"nomer" validate:"required,numeric"`
	Language string `form:"language" json:"language" validate:"required"`
}
