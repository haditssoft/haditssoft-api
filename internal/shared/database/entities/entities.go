package entities

import (
	"database/sql"
	"time"

	"gorm.io/gorm"
)

type User struct {
	ID                        uint           `gorm:"column:ID;primaryKey;autoIncrement" json:"id"`
	Email                     string         `gorm:"column:Email;notNull;size:50" json:"email"`
	Password                  string         `gorm:"column:Password;notNull" json:"-"`
	VerificationCode          string         `gorm:"column:VerificationCode;size:10" json:"-"`
	VerificationCodeExpiresAt sql.NullTime   `gorm:"column:VerificationCodeExpiresAt" json:"-"`
	EmailVerifiedAt           sql.NullTime   `gorm:"column:EmailVerifiedAt" json:"-"`
	Active                    *bool          `gorm:"column:Active;notNull;default:0" json:"active"`
	Admin                     *bool          `gorm:"column:Admin;notNull;default:0" json:"admin"`
	CreatedBy                 string         `gorm:"column:CreatedBy;size:50" json:"created_by"`
	UpdatedBy                 string         `gorm:"column:UpdatedBy;size:50" json:"updated_by"`
	CreatedAt                 time.Time      `gorm:"column:CreatedAt" json:"created_at"`
	UpdatedAt                 time.Time      `gorm:"column:UpdatedAt" json:"updated_at"`
	DeletedAt                 gorm.DeletedAt `gorm:"column:DeletedAt;index" json:"deleted_at"`
	Activities                []Activity     `gorm:"foreignKey:UserID" json:"activities,omitempty"`
}

func (User) TableName() string { return "User" }

type Activity struct {
	ID          uint      `gorm:"column:ID;primaryKey;autoIncrement" json:"id"`
	UserID      uint      `gorm:"column:UserID;notNull" json:"user_id"`
	ReferenceID uint      `gorm:"column:ReferenceID" json:"reference_id"`
	ActionURL   string    `gorm:"column:ActionURL;size:255" json:"action_url"`
	ReqMethod   string    `gorm:"column:ReqMethod;size:10" json:"req_method"`
	Note        string    `gorm:"column:Note;size:255" json:"note"`
	IP          string    `gorm:"column:IP;size:50" json:"ip"`
	CreatedAt   time.Time `gorm:"column:CreatedAt" json:"created_at"`
	UpdatedAt   time.Time `gorm:"column:UpdatedAt" json:"updated_at"`
	User        User      `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

func (Activity) TableName() string { return "Activity" }

type BlacklistToken struct {
	ID        uint      `gorm:"column:ID;primaryKey;autoIncrement" json:"id"`
	Token     string    `gorm:"column:Token;size:500;notNull" json:"token"`
	CreatedAt time.Time `gorm:"column:CreatedAt" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:UpdatedAt" json:"updated_at"`
}

func (BlacklistToken) TableName() string { return "BlacklistToken" }

type RefreshToken struct {
	ID        uint      `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	UserID    uint      `gorm:"column:user_id;notNull" json:"user_id"`
	TokenHash string    `gorm:"column:token_hash;size:64;notNull" json:"-"`
	IsUsed    bool      `gorm:"column:is_used;notNull;default:false" json:"is_used"`
	ExpiresAt time.Time `gorm:"column:expires_at;notNull" json:"expires_at"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
}

func (RefreshToken) TableName() string { return "RefreshToken" }

type TranslationSetting struct {
	ID        uint      `gorm:"column:id;primaryKey;autoIncrement;type:integer" json:"id"`
	UserID    uint      `gorm:"column:user_id;type:integer;not null;uniqueIndex" json:"user_id"`
	Language  string    `gorm:"column:language;type:text;not null" json:"language"`
	CreatedAt time.Time `gorm:"column:created_at;type:datetime;not null" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at;type:datetime;not null" json:"updated_at"`
}

func (TranslationSetting) TableName() string { return "TranslationSetting" }
