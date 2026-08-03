package response

import "time"

type UserResponseField struct {
	ID        uint      `json:"id"`
	Email     string    `json:"email"`
	Active    *bool     `json:"active"`
	Admin     *bool     `json:"admin"`
	CreatedBy string    `json:"created_by"`
	UpdatedBy string    `json:"updated_by"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
