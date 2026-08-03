package database_test

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/haditssoft/haditssoft-backend/internal/shared/database/entities"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

var testDBCounter int

func setupSchemaDriftDB(t *testing.T) *gorm.DB {
	t.Helper()
	testDBCounter++
	dbName := fmt.Sprintf("file:schema_drift_%d?mode=memory&cache=shared", testDBCounter)
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
	t.Cleanup(func() {
		sqlDB, _ := db.DB()
		sqlDB.Close()
	})
	return db
}

func createProductionUserSchema(db *gorm.DB) {
	db.Exec(`
		CREATE TABLE IF NOT EXISTS "User" (
			"ID" integer NOT NULL PRIMARY KEY,
			"Email" text NOT NULL UNIQUE,
			"Password" text NOT NULL,
			"EmailVerifiedAt" datetime,
			"Active" numeric NOT NULL DEFAULT false,
			"Admin" numeric NOT NULL DEFAULT false,
			"CreatedBy" text,
			"UpdatedBy" text,
			"CreatedAt" datetime,
			"UpdatedAt" datetime,
			"DeletedAt" datetime,
			"VerificationCode" text,
			"VerificationCodeExpiresAt" datetime
		)
	`)
	db.Exec(`CREATE INDEX IF NOT EXISTS "idx_User_DeletedAt" ON "User"("DeletedAt")`)
}

func createProductionActivitySchema(db *gorm.DB) {
	db.Exec(`
		CREATE TABLE IF NOT EXISTS "Activity" (
			"ID" integer NOT NULL PRIMARY KEY,
			"UserID" integer NOT NULL,
			"ReferenceID" integer NOT NULL DEFAULT 0,
			"ActionURL" text NOT NULL DEFAULT '',
			"ReqMethod" text NOT NULL DEFAULT '',
			"Note" text NOT NULL DEFAULT '',
			"IP" text NOT NULL DEFAULT '',
			"CreatedAt" datetime,
			"UpdatedAt" datetime
		)
	`)
}

func createProductionBlacklistTokenSchema(db *gorm.DB) {
	db.Exec(`
		CREATE TABLE IF NOT EXISTS "BlacklistToken" (
			"ID" integer NOT NULL PRIMARY KEY,
			"Token" text NOT NULL,
			"CreatedAt" datetime,
			"UpdatedAt" datetime
		)
	`)
	db.Exec(`CREATE INDEX IF NOT EXISTS "idx_BlacklistToken_Token" ON "BlacklistToken"("Token")`)
}

func createProductionRefreshTokenSchema(db *gorm.DB) {
	db.Exec(`
		CREATE TABLE IF NOT EXISTS "RefreshToken" (
			"id" integer NOT NULL PRIMARY KEY,
			"user_id" integer NOT NULL,
			"token_hash" text NOT NULL,
			"is_used" numeric NOT NULL DEFAULT false,
			"expires_at" datetime NOT NULL,
			"created_at" datetime NOT NULL
		)
	`)
	db.Exec(`CREATE INDEX IF NOT EXISTS "idx_RefreshToken_TokenHash" ON "RefreshToken"("token_hash")`)
}

func createAllProductionSchemas(db *gorm.DB) {
	createProductionUserSchema(db)
	createProductionActivitySchema(db)
	createProductionBlacklistTokenSchema(db)
	createProductionRefreshTokenSchema(db)
}

func getTableColumns(t *testing.T, db *gorm.DB, tableName string) []string {
	t.Helper()
	var columns []string
	rows, err := db.Raw(fmt.Sprintf("PRAGMA table_info(%q)", tableName)).Rows()
	if err != nil {
		t.Fatalf("failed to get table info for %s: %v", tableName, err)
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull int
		var dfltValue interface{}
		var pk int
		rows.Scan(&cid, &name, &ctype, &notnull, &dfltValue, &pk)
		columns = append(columns, name)
	}
	return columns
}

// TestEntityColumnTags_MatchProductionSchema verifies that entity struct column tags
// match the actual production database columns. This catches the exact bug where
// entities use snake_case column tags (e.g. column:user_id) but the production DB
// has PascalCase columns (e.g. UserID) created by old models.
//
// If this test fails, it means an entity's column tag doesn't match what the
// production DB actually stores, which will cause silent INSERT/SELECT failures.
func TestEntityColumnTags_MatchProductionSchema(t *testing.T) {
	db := setupSchemaDriftDB(t)
	createAllProductionSchemas(db)

	type entityTest struct {
		name      string
		tableName string
		model     interface{}
		factory   func() interface{}
		populate  func(db *gorm.DB, m interface{})
		verifiers []func(t *testing.T, db *gorm.DB)
	}

	tests := []entityTest{
		{
			name:      "User",
			tableName: "User",
			model:     &entities.User{},
			factory: func() interface{} {
				active := true
				admin := false
				return &entities.User{
					Email:    "test@example.com",
					Password: "hashedpassword",
					Active:   &active,
					Admin:    &admin,
				}
			},
			populate: func(db *gorm.DB, m interface{}) {
				db.Exec(`INSERT INTO "User" ("Email","Password","Active","Admin","CreatedAt","UpdatedAt") VALUES (?,?,?,?,?,?)`,
					"test@example.com", "hashedpassword", 1, 0, time.Now(), time.Now())
			},
			verifiers: []func(t *testing.T, db *gorm.DB){
				func(t *testing.T, db *gorm.DB) {
					var user entities.User
					result := db.Where("Email = ?", "test@example.com").First(&user)
					if result.Error != nil {
						t.Fatalf("SELECT by Email failed: %v", result.Error)
					}
					if user.Email != "test@example.com" {
						t.Errorf("Email = %q, want %q", user.Email, "test@example.com")
					}
				},
				func(t *testing.T, db *gorm.DB) {
					active := true
					admin := false
					user := &entities.User{
						Email:    "select_test@example.com",
						Password: "pw",
						Active:   &active,
						Admin:    &admin,
					}
					if err := db.Create(user).Error; err != nil {
						t.Fatalf("INSERT failed: %v", err)
					}
					var fetched entities.User
					if err := db.Where("id = ?", user.ID).First(&fetched).Error; err != nil {
						t.Fatalf("SELECT by ID failed after INSERT: %v", err)
					}
					if fetched.Email != "select_test@example.com" {
						t.Errorf("roundtrip Email = %q, want %q", fetched.Email, "select_test@example.com")
					}
				},
				func(t *testing.T, db *gorm.DB) {
					if err := db.Where("Email = ?", "test@example.com").Delete(&entities.User{}).Error; err != nil {
						t.Fatalf("soft DELETE failed: %v", err)
					}
					var count int64
					db.Model(&entities.User{}).Where("Email = ?", "test@example.com").Count(&count)
					if count != 0 {
						t.Errorf("soft-deleted user still visible: count = %d", count)
					}
				},
			},
		},
		{
			name:      "Activity",
			tableName: "Activity",
			model:     &entities.Activity{},
			factory: func() interface{} {
				return &entities.Activity{
					UserID:    1,
					ActionURL: "/test",
					ReqMethod: "POST",
					Note:      "Test activity",
					IP:        "127.0.0.1",
				}
			},
			populate: func(db *gorm.DB, m interface{}) {
				db.Exec(`INSERT INTO "Activity" ("UserID","ActionURL","ReqMethod","Note","IP","CreatedAt") VALUES (?,?,?,?,?,?)`,
					1, "/login", "POST", "Login", "127.0.0.1", time.Now())
			},
			verifiers: []func(t *testing.T, db *gorm.DB){
				func(t *testing.T, db *gorm.DB) {
					var act entities.Activity
					result := db.Where("Note = ?", "Login").First(&act)
					if result.Error != nil {
						t.Fatalf("SELECT Activity by Note failed: %v", result.Error)
					}
					if act.ReqMethod != "POST" {
						t.Errorf("ReqMethod = %q, want %q", act.ReqMethod, "POST")
					}
					if act.UserID != 1 {
						t.Errorf("UserID = %d, want 1", act.UserID)
					}
				},
				func(t *testing.T, db *gorm.DB) {
					act := &entities.Activity{
						UserID:    1,
						ActionURL: "/roundtrip",
						ReqMethod: "GET",
						Note:      "Roundtrip test",
						IP:        "127.0.0.1",
					}
					if err := db.Create(act).Error; err != nil {
						t.Fatalf("INSERT Activity failed: %v", err)
					}
					var fetched entities.Activity
					if err := db.Where("id = ?", act.ID).First(&fetched).Error; err != nil {
						t.Fatalf("SELECT Activity after INSERT failed: %v", err)
					}
					if fetched.Note != "Roundtrip test" {
						t.Errorf("roundtrip Note = %q, want %q", fetched.Note, "Roundtrip test")
					}
					if fetched.ActionURL != "/roundtrip" {
						t.Errorf("roundtrip ActionURL = %q, want %q", fetched.ActionURL, "/roundtrip")
					}
				},
			},
		},
		{
			name:      "BlacklistToken",
			tableName: "BlacklistToken",
			model:     &entities.BlacklistToken{},
			factory: func() interface{} {
				return &entities.BlacklistToken{Token: "test-token-abc"}
			},
			populate: func(db *gorm.DB, m interface{}) {
				db.Exec(`INSERT INTO "BlacklistToken" ("Token","CreatedAt") VALUES (?,?)`,
					"seed-token", time.Now())
			},
			verifiers: []func(t *testing.T, db *gorm.DB){
				func(t *testing.T, db *gorm.DB) {
					var bt entities.BlacklistToken
					result := db.Where("Token = ?", "seed-token").First(&bt)
					if result.Error != nil {
						t.Fatalf("SELECT BlacklistToken by Token failed: %v", result.Error)
					}
					if bt.Token != "seed-token" {
						t.Errorf("Token = %q, want %q", bt.Token, "seed-token")
					}
				},
				func(t *testing.T, db *gorm.DB) {
					bt := &entities.BlacklistToken{Token: "roundtrip-token"}
					if err := db.Create(bt).Error; err != nil {
						t.Fatalf("INSERT BlacklistToken failed: %v", err)
					}
					var fetched entities.BlacklistToken
					if err := db.Where("id = ?", bt.ID).First(&fetched).Error; err != nil {
						t.Fatalf("SELECT BlacklistToken after INSERT failed: %v", err)
					}
					if fetched.Token != "roundtrip-token" {
						t.Errorf("roundtrip Token = %q, want %q", fetched.Token, "roundtrip-token")
					}
				},
			},
		},
		{
			name:      "RefreshToken",
			tableName: "RefreshToken",
			model:     &entities.RefreshToken{},
			factory: func() interface{} {
				return &entities.RefreshToken{
					UserID:    1,
					TokenHash: "hash123",
					ExpiresAt: time.Now().Add(time.Hour),
				}
			},
			populate: func(db *gorm.DB, m interface{}) {
				db.Exec(`INSERT INTO "RefreshToken" ("user_id","token_hash","is_used","expires_at","created_at") VALUES (?,?,?,?,?)`,
					1, "seed-hash", 0, time.Now().Add(time.Hour), time.Now())
			},
			verifiers: []func(t *testing.T, db *gorm.DB){
				func(t *testing.T, db *gorm.DB) {
					var rt entities.RefreshToken
					result := db.Where("token_hash = ?", "seed-hash").First(&rt)
					if result.Error != nil {
						t.Fatalf("SELECT RefreshToken by token_hash failed: %v", result.Error)
					}
					if rt.TokenHash != "seed-hash" {
						t.Errorf("TokenHash = %q, want %q", rt.TokenHash, "seed-hash")
					}
				},
				func(t *testing.T, db *gorm.DB) {
					rt := &entities.RefreshToken{
						UserID:    1,
						TokenHash: "roundtrip-hash",
						ExpiresAt: time.Now().Add(time.Hour),
					}
					if err := db.Create(rt).Error; err != nil {
						t.Fatalf("INSERT RefreshToken failed: %v", err)
					}
					var fetched entities.RefreshToken
					if err := db.Where("id = ?", rt.ID).First(&fetched).Error; err != nil {
						t.Fatalf("SELECT RefreshToken after INSERT failed: %v", err)
					}
					if fetched.TokenHash != "roundtrip-hash" {
						t.Errorf("roundtrip TokenHash = %q, want %q", fetched.TokenHash, "roundtrip-hash")
					}
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dbColumns := getTableColumns(t, db, tc.tableName)
			dbColumnSet := make(map[string]bool)
			for _, c := range dbColumns {
				dbColumnSet[strings.ToLower(c)] = true
			}

			modelType := reflect.TypeOf(tc.model).Elem()
			for i := 0; i < modelType.NumField(); i++ {
				field := modelType.Field(i)
				gormTag := field.Tag.Get("gorm")
				if gormTag == "" || field.Type.Name() == "" {
					continue
				}
				if field.Type == reflect.TypeOf(gorm.DeletedAt{}) {
					continue
				}

				colName := extractColumnTag(gormTag)
				if colName == "" {
					continue
				}

				if !dbColumnSet[strings.ToLower(colName)] {
					t.Errorf("entity field %s has column:%s but production table %q has no matching column (db columns: %v)",
						field.Name, colName, tc.tableName, dbColumns)
				}
			}

			if tc.populate != nil {
				tc.populate(db, nil)
			}

			for i, verify := range tc.verifiers {
				t.Run(fmt.Sprintf("verifier_%d", i), func(t *testing.T) {
					verify(t, db)
				})
			}
		})
	}
}

func extractColumnTag(gormTag string) string {
	for _, part := range strings.Split(gormTag, ";") {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "column:") {
			return strings.TrimPrefix(part, "column:")
		}
	}
	return ""
}
