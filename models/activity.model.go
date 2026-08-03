package models

import "time"

type Activity struct {
	ID          uint      `gorm:"primaryKey;notNull;" json:"id"`
	UserID      uint      `gorm:"notNull;comment:user id" json:"user_id"`
	ReferenceID uint      `gorm:"notNull;index;comment:uuid table lain yang di create/update/delete" json:"reference_id"`
	ActionURL   string    `gorm:"notNull" json:"action_url"`
	ReqMethod   string    `gorm:"notNull;size:10" json:"req_method"`
	Note        string    `gorm:"notNull" json:"note"`
	IP          string    `gorm:"notNull;size:60" json:"ip"`
	CreatedAt   time.Time `json:"created_at"`
	User        User      `gorm:"constraint:OnDelete:CASCADE;" json:"user"`
}
