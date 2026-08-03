package models

import (
	"time"
)

type SearchMode struct {
	ID         uint      `gorm:"column:id;primaryKey;autoIncrement;type:integer" json:"id"`
	UserID     uint      `gorm:"column:user_id;type:integer;not null;uniqueIndex" json:"user_id"`
	SearchMode uint      `gorm:"column:search_mode;type:integer;not null" json:"search_mode"`
	CreatedAt  time.Time `gorm:"column:created_at;type:datetime;not null" json:"created_at"`
	UpdatedAt  time.Time `gorm:"column:updated_at;type:datetime;not null" json:"updated_at"`
}

func (SearchMode) TableName() string {
	return "SearchMode"
}
