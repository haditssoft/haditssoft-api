package models

import (
	"time"
)

type TranslationSetting struct {
	ID        uint      `gorm:"column:id;primaryKey;autoIncrement;type:integer" json:"id"`
	UserID    uint      `gorm:"column:user_id;type:integer;not null;uniqueIndex" json:"user_id"`
	Language  string    `gorm:"column:language;type:text;not null" json:"language"`
	CreatedAt time.Time `gorm:"column:created_at;type:datetime;not null" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at;type:datetime;not null" json:"updated_at"`
}

func (TranslationSetting) TableName() string {
	return "TranslationSetting"
}
