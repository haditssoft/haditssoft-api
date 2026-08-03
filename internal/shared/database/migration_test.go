package database

import (
	"fmt"
	"os"
	"testing"

	"github.com/haditssoft/haditssoft-backend/internal/shared/database/entities"

	"github.com/glebarez/sqlite"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

var migrationDBCounter int

func setupMigrationTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	migrationDBCounter++
	dbName := fmt.Sprintf("file:migration_%d?mode=memory&cache=shared", migrationDBCounter)
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{
		SkipDefaultTransaction:                   true,
		DisableForeignKeyConstraintWhenMigrating: true,
		NamingStrategy: schema.NamingStrategy{
			SingularTable: true,
			NoLowerCase:   true,
		},
	})
	if err != nil {
		t.Fatalf("failed to open test DB: %v", err)
	}

	db.AutoMigrate(&entities.User{})

	t.Cleanup(func() {
		sqlDB, _ := db.DB()
		sqlDB.Close()
	})

	return db
}

func TestRunMigrations_CreatesAdminWithHashedPassword(t *testing.T) {
	origDB := DB
	DB = setupMigrationTestDB(t)
	t.Cleanup(func() { DB = origDB })

	os.Setenv("ADMIN_EMAIL", "testadmin@example.com")
	os.Setenv("ADMIN_PASSWORD", "MyS3cur3P@ss!")
	t.Cleanup(func() {
		os.Unsetenv("ADMIN_EMAIL")
		os.Unsetenv("ADMIN_PASSWORD")
	})

	RunMigrations()

	var user entities.User
	result := DB.Where("email = ?", "testadmin@example.com").First(&user)
	if result.Error != nil {
		t.Fatalf("admin user not found: %v", result.Error)
	}

	if user.Password == "" {
		t.Fatal("password hash is empty")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte("MyS3cur3P@ss!")); err != nil {
		t.Errorf("password hash does not match the plaintext password: %v", err)
	}

	if user.Active == nil || !*user.Active {
		t.Error("admin user should be active")
	}
	if user.Admin == nil || !*user.Admin {
		t.Error("admin user should have admin = true")
	}
}

func TestRunMigrations_Idempotent(t *testing.T) {
	origDB := DB
	DB = setupMigrationTestDB(t)
	t.Cleanup(func() { DB = origDB })

	os.Setenv("ADMIN_EMAIL", "admin@example.com")
	os.Setenv("ADMIN_PASSWORD", "somepassword")
	t.Cleanup(func() {
		os.Unsetenv("ADMIN_EMAIL")
		os.Unsetenv("ADMIN_PASSWORD")
	})

	RunMigrations()

	var count int64
	DB.Model(&entities.User{}).Count(&count)
	if count != 1 {
		t.Fatalf("expected 1 user after first migration, got %d", count)
	}

	RunMigrations()

	DB.Model(&entities.User{}).Count(&count)
	if count != 1 {
		t.Errorf("expected still 1 user after second migration (idempotent), got %d", count)
	}
}

func TestRunMigrations_PanicsWithoutAdminEmail(t *testing.T) {
	origDB := DB
	DB = setupMigrationTestDB(t)
	t.Cleanup(func() { DB = origDB })

	os.Unsetenv("ADMIN_EMAIL")
	os.Setenv("ADMIN_PASSWORD", "somepassword")
	t.Cleanup(func() { os.Unsetenv("ADMIN_PASSWORD") })

	defer func() {
		r := recover()
		if r == nil {
			t.Error("expected panic when ADMIN_EMAIL is not set, but none occurred")
		}
	}()

	RunMigrations()
}

func TestRunMigrations_PanicsWithoutAdminPassword(t *testing.T) {
	origDB := DB
	DB = setupMigrationTestDB(t)
	t.Cleanup(func() { DB = origDB })

	os.Unsetenv("ADMIN_PASSWORD")
	os.Setenv("ADMIN_EMAIL", "admin@example.com")
	t.Cleanup(func() { os.Unsetenv("ADMIN_EMAIL") })

	defer func() {
		r := recover()
		if r == nil {
			t.Error("expected panic when ADMIN_PASSWORD is not set, but none occurred")
		}
	}()

	RunMigrations()
}
