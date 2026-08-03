package models

import (
	"time"
)

type Font struct {
	ID                  uint      `gorm:"column:id;primaryKey;autoIncrement;type:integer" json:"id"`
	UserID              uint      `gorm:"column:user_id;type:integer;not null;uniqueIndex" json:"user_id"`
	ArabicFamily        string    `gorm:"column:arabic_family;type:text;not null" json:"arabic_family"`
	ArabicFallback      string    `gorm:"column:arabic_fallback;type:text;not null" json:"arabic_fallback"`
	ArabicWeight        int       `gorm:"column:arabic_weight;type:integer;not null" json:"arabic_weight"`
	ArabicSize          int       `gorm:"column:arabic_size;type:integer;not null" json:"arabic_size"`
	TranslationFamily   string    `gorm:"column:translation_family;type:text;not null" json:"translation_family"`
	TranslationFallback string    `gorm:"column:translation_fallback;type:text;not null" json:"translation_fallback"`
	TranslationWeight   int       `gorm:"column:translation_weight;type:integer;not null" json:"translation_weight"`
	TranslationSize     int       `gorm:"column:translation_size;type:integer;not null" json:"translation_size"`
	CreatedAt           time.Time `gorm:"column:created_at;type:datetime;not null" json:"created_at"`
	UpdatedAt           time.Time `gorm:"column:updated_at;type:datetime;not null" json:"updated_at"`
}

func (Font) TableName() string {
	return "Font"
}
