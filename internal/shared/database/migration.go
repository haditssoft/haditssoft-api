package database

import (
	"os"

	"github.com/haditssoft/haditssoft-backend/internal/shared/database/entities"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func RunMigrations() {
	DB.AutoMigrate(
		&entities.User{},
		&entities.Activity{},
		&entities.BlacklistToken{},
		&entities.RefreshToken{},
	)

	email := os.Getenv("ADMIN_EMAIL")
	if email == "" {
		panic("ADMIN_EMAIL environment variable is not set")
	}

	password := os.Getenv("ADMIN_PASSWORD")
	if password == "" {
		panic("ADMIN_PASSWORD environment variable is not set")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), 14)
	if err != nil {
		panic("Failed to hash admin password")
	}

	var user entities.User
	result := DB.Select("id").Unscoped().Where("email = ?", email).First(&user)
	if result.Error != nil {
		isActive := true
		model := entities.User{
			Active:   &isActive,
			Admin:    &isActive,
			Email:    email,
			Password: string(hash),
		}

		if err := DB.Session(&gorm.Session{SkipHooks: true}).Omit(clause.Associations).Create(&model).Error; err != nil {
			panic("Failed add initial user")
		}
	}
}
