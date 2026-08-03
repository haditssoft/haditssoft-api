package opencode

type AskOpenCodeRequest struct {
	Prompt     string `form:"prompt" json:"prompt" validate:"required,min=1"`
	System     string `form:"system" json:"system"`
	ProviderID string `form:"provider_id" json:"provider_id"`
	ModelID    string `form:"model_id" json:"model_id"`
}
