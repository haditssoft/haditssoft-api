package models

import (
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/schema"
)

var activityDBCounter int

func setupActivityTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	activityDBCounter++
	dbName := fmt.Sprintf("file:activity_%d?mode=memory&cache=shared", activityDBCounter)
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
	if err := db.AutoMigrate(&Activity{}); err != nil {
		t.Fatalf("failed to migrate Activity: %v", err)
	}
	t.Cleanup(func() {
		sqlDB, _ := db.DB()
		sqlDB.Close()
	})
	return db
}

func TestActivity_UsesActivityTableName(t *testing.T) {
	db := setupActivityTestDB(t)

	if !db.Migrator().HasTable("Activity") {
		t.Error("expected an 'Activity' table to be created by GORM naming strategy")
	}
}

func TestActivity_CreateAndFindRoundTrip(t *testing.T) {
	db := setupActivityTestDB(t)

	act := Activity{
		UserID:      1,
		ReferenceID: 99,
		ActionURL:   "/admin/users",
		ReqMethod:   "POST",
		Note:        "Create new user",
		IP:          "127.0.0.1",
	}

	if err := db.Omit(clause.Associations).Create(&act).Error; err != nil {
		t.Fatalf("failed to create activity: %v", err)
	}
	if act.ID == 0 {
		t.Fatal("expected a generated ID after create")
	}

	var got Activity
	if err := db.First(&got, act.ID).Error; err != nil {
		t.Fatalf("failed to find activity: %v", err)
	}

	if got.UserID != 1 || got.ReferenceID != 99 {
		t.Errorf("round-trip user/reference mismatch: %+v", got)
	}
	if got.ActionURL != "/admin/users" || got.ReqMethod != "POST" {
		t.Errorf("round-trip request info mismatch: %+v", got)
	}
	if got.Note != "Create new user" || got.IP != "127.0.0.1" {
		t.Errorf("round-trip note/ip mismatch: %+v", got)
	}
}
