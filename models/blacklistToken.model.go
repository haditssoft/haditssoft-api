package models

import (
	"time"

	"gorm.io/gorm"
)

type BlacklistToken struct {
	ID        uint      `gorm:"primaryKey;notNull;" json:"id"`
	Token     string    `gorm:"notNull;index" json:"token"`
	CreatedAt time.Time `json:"created_at"`
}

func (m *BlacklistToken) BeforeCreate(tx *gorm.DB) (err error) {
	// GENERATE UUID
	// uuidx, err := uuid.NewRandom()
	// if err != nil {
	// 	return
	// }
	// tx.Statement.SetColumn("ID", uuidx.String())
	// GENERATE UUID
	// PERTAMA BUAT IS SYNCED FALSE
	// tx.Statement.SetColumn("IsSynced", false)
	// PERTAMA BUAT IS SYNCED FALSE
	return
}
