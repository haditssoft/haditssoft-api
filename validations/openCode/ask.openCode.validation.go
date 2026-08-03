package openCode

type AskOpenCode struct {
	Prompt string `form:"prompt" json:"prompt" validate:"required,min=1"`
	System string `form:"system" json:"system"`
}
