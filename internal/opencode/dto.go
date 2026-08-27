package opencode

type AskOpenCodeRequest struct {
	Prompt string `form:"prompt" json:"prompt" validate:"required,min=1"`
}
