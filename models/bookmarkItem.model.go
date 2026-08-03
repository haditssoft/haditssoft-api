package models

import (
	"time"

	"gorm.io/gorm"
)

type BookmarkItem struct {
	ID         uint           `gorm:"column:id;primaryKey;notNull;" json:"id"`
	BookmarkID uint           `gorm:"column:bookmark_id;notNull;comment:parent id" json:"bookmark_id"`
	BookName   string         `gorm:"column:book_name;notNull" json:"book_name"`
	BookNumber string         `gorm:"column:book_number;notNull" json:"book_number"`
	CreatedAt  time.Time      `gorm:"column:created_at;type:datetime;notNull" json:"created_at"`
	DeletedAt  gorm.DeletedAt `json:"-" gorm:"column:deleted_at;type:datetime;index"`
	Bookmark   Bookmark       `json:"bookmark"`
}

func (BookmarkItem) TableName() string {
	return "bookmark_item"
}
