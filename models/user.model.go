package models

import (
	"database/sql"
	"github.com/haditssoft/haditssoft-backend/internal/shared/database"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type User struct {
	ID                        uint           `gorm:"primaryKey;notNull;" json:"id"`
	Email                     string         `gorm:"notNull;uniqueIndex;size:50" json:"email"`
	Password                  string         `gorm:"notNull" json:"-"`
	EmailVerifiedAt           sql.NullTime   `json:"-"`
	VerificationCode          string         `gorm:"size:6" json:"-"`
	VerificationCodeExpiresAt sql.NullTime   `json:"-"`
	Active                    *bool          `gorm:"default:false;notNull" json:"active"`
	Admin                     *bool          `gorm:"default:false;notNull" json:"admin"`
	CreatedBy                 string         `gorm:"size:36;comment:isi dengan uuid" json:"created_by"`
	UpdatedBy                 string         `gorm:"size:36;comment:isi dengan uuid" json:"updated_by"`
	CreatedAt                 time.Time      `json:"created_at"`
	UpdatedAt                 time.Time      `json:"updated_at"`
	DeletedAt                 gorm.DeletedAt `gorm:"index" json:"-"`
	Activities                []Activity     `json:"activities"`
}

func (User) TableName() string {
	return "User"
}

func SearchUser(page, limit int, search string) ([]User, int64, error) {
	var rows []User
	var total int64

	kitab := "User"
	db := database.DB.Table(kitab)

	if search != "" {
		db = db.Where("Email LIKE ?", "%"+search+"%")
	}

	db.Count(&total)

	offset := (page - 1) * limit
	err := db.Select(
		"ID",
		"Email",
		"Password",
		"EmailVerifiedAt",
		"VerificationCode",
		"VerificationCodeExpiresAt",
		"Active",
		"Admin",
		"CreatedBy",
		"UpdatedBy",
		"CreatedAt",
		"UpdatedAt",
	).Offset(offset).Limit(limit).Find(&rows).Error

	if err != nil {
		return nil, 0, err
	}

	return rows, total, nil
}

// HOOKS

func (m *User) AfterFind(tx *gorm.DB) (err error) {
	// BIRTHDATE
	// v, err := m.BirthDate.Value()
	// if err == nil {
	// 	date, err := time.Parse("2006-01-02", v.(time.Time).String())
	// 	if err != nil {
	// 		return err
	// 	}
	// 	m.BirthDate = sql.NullTime{Time: date, Valid: true}
	// }
	// GENDER
	// if m.Gender == "M" {
	// 	m.Gender = "Male"
	// } else if m.Gender == "F" {
	// 	m.Gender = "Female"
	// }

	return
}

func (m *User) AfterCreate(tx *gorm.DB) (err error) {
	// saveFile(m)

	// // update kolom image yang masih null dengan data json
	// tx.Model(m).UpdateColumn("image", m.Image)

	return
}

// Updating data in same transaction
func (m *User) AfterUpdate(tx *gorm.DB) (err error) {
	return //errors.New("hai pokpand")
}

func (m *User) AfterSave(tx *gorm.DB) (err error) {
	return
}

func (m *User) AfterDelete(tx *gorm.DB) (err error) {
	return
}

func (m *User) BeforeSave(tx *gorm.DB) (err error) {
	trx := tx.Statement.Dest.(*User)

	// HASH PASSWORD
	if len(trx.Password) > 0 {

		bytes, errod := bcrypt.GenerateFromPassword([]byte(trx.Password), 14)
		if errod != nil {
			err = errod
			return
		}
		tx.Statement.SetColumn("Password", string(bytes))
	}
	// HASH PASSWORD
	return
}

func (m *User) BeforeCreate(tx *gorm.DB) (err error) {
	if tx.Statement.Context.Value("skip_before_create") == true {
		return nil
	}
	// GENERATE UUID
	// uuidx, err := uuid.NewRandom()
	// if err != nil {
	// 	return
	// }
	// tx.Statement.SetColumn("ID", uuidx.String())
	// GENERATE UUID
	// trx := tx.Statement.Dest.(*User)

	// CREATED BY
	tx.Statement.SetColumn("CreatedBy", tx.Statement.Context.Value("user_id").(float64))
	// CREATED BY
	return
}

func (m *User) BeforeUpdate(tx *gorm.DB) (err error) {
	// IsSynced rubah ke false hanya jika
	// newValue true
	// dan di database juga true
	// CREATED BY
	tx.Statement.SetColumn("UpdatedBy", tx.Statement.Context.Value("user_id").(float64))
	// CREATED BY
	return
}

// Model Method
func (m *User) TotalRecords() (count int64) {
	database.DB.Model(&User{}).Count(&count)
	return
}
