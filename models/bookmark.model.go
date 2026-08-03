package models

import (
	"time"

	"gorm.io/gorm"
)

type Bookmark struct {
	ID           uint           `gorm:"column:id;primaryKey;notNull;" json:"id"`
	UserID       uint           `gorm:"column:user_id;notNull;comment:user id" json:"user_id"`
	Title        string         `gorm:"column:title;notNull" json:"title"`
	CreatedAt    time.Time      `gorm:"column:created_at;type:datetime;notNull" json:"created_at"`
	UpdatedAt    time.Time      `gorm:"column:updated_at;type:datetime;notNull" json:"updated_at"`
	DeletedAt    gorm.DeletedAt `json:"-" gorm:"column:deleted_at;type:datetime;index"`
	BookmarkItem []BookmarkItem `gorm:"constraint:OnDelete:CASCADE;foreignKey:BookmarkID" json:"bookmark_item"`
}

func (Bookmark) TableName() string {
	return "bookmark"
}
