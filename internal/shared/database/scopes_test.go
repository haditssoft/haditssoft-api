package database

import (
	"fmt"
	"strings"
	"testing"

	"github.com/haditssoft/haditssoft-backend/internal/shared/database/entities"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

var scopesDBCounter int

func setupScopesTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	scopesDBCounter++
	dbName := fmt.Sprintf("file:scopes_%d?mode=memory&cache=shared", scopesDBCounter)
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
	if err := db.AutoMigrate(&entities.User{}); err != nil {
		t.Fatalf("failed to migrate User: %v", err)
	}
	t.Cleanup(func() {
		sqlDB, _ := db.DB()
		sqlDB.Close()
	})
	return db
}

func TestLimitRangeSort_AppliesOffsetAndLimit(t *testing.T) {
	db := setupScopesTestDB(t)

	var users []entities.User
	query := db.ToSQL(func(tx *gorm.DB) *gorm.DB {
		return tx.Model(&entities.User{}).Scopes(LimitRangeSort([]int{10, 19}, nil)).Find(&users)
	})

	if !strings.Contains(query, "LIMIT 10") {
		t.Errorf("expected LIMIT 10 in query, got: %s", query)
	}
	if !strings.Contains(query, "OFFSET 10") {
		t.Errorf("expected OFFSET 10 in query, got: %s", query)
	}
}

func TestLimitRangeSort_AppliesOrderBy(t *testing.T) {
	db := setupScopesTestDB(t)

	var users []entities.User
	query := db.ToSQL(func(tx *gorm.DB) *gorm.DB {
		return tx.Model(&entities.User{}).Scopes(LimitRangeSort(nil, []string{"ID desc"})).Find(&users)
	})

	if !strings.Contains(query, "ID desc") {
		t.Errorf("expected ORDER BY ID desc in query, got: %s", query)
	}
}

func TestLimitRangeSort_EmptyRangesSkipsLimit(t *testing.T) {
	db := setupScopesTestDB(t)

	var users []entities.User
	query := db.ToSQL(func(tx *gorm.DB) *gorm.DB {
		return tx.Model(&entities.User{}).Scopes(LimitRangeSort(nil, nil)).Find(&users)
	})

	if strings.Contains(query, "LIMIT") {
		t.Errorf("expected no LIMIT clause when ranges are empty, got: %s", query)
	}
}

func TestLimitRangeSort_SingleElementRangeUsesLimitOne(t *testing.T) {
	db := setupScopesTestDB(t)

	var users []entities.User
	query := db.ToSQL(func(tx *gorm.DB) *gorm.DB {
		return tx.Model(&entities.User{}).Scopes(LimitRangeSort([]int{5}, nil)).Find(&users)
	})

	if strings.Contains(query, "LIMIT") {
		t.Errorf("expected no LIMIT clause for a single-element range, got: %s", query)
	}
}
