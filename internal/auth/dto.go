package auth

type LoginInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type RefreshInput struct {
	RefreshToken string `json:"refresh_token"`
}

type LoginResponse struct {
	Status       string `json:"status"`
	Message      string `json:"message"`
	Token        string `json:"token"`
	RefreshToken string `json:"refresh_token"`
}

type IdentityResponse struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}
