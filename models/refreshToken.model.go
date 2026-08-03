package models

import "time"

type RefreshToken struct {
	ID        uint      `gorm:"column:id;primaryKey;notNull;" json:"id"`
	UserID    uint      `gorm:"column:user_id;notNull;index" json:"user_id"`
	TokenHash string    `gorm:"column:token_hash;notNull;index" json:"-"`
	IsUsed    bool      `gorm:"column:is_used;default:false;notNull" json:"is_used"`
	ExpiresAt time.Time `gorm:"column:expires_at;notNull" json:"expires_at"`
	CreatedAt time.Time `gorm:"column:created_at;notNull" json:"created_at"`
}

func (RefreshToken) TableName() string {
	return "RefreshToken"
}
