package validator

import (
	"testing"

	"github.com/haditssoft/haditssoft-backend/internal/shared/database"
	"github.com/haditssoft/haditssoft-backend/internal/shared/database/entities"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

type basicValidation struct {
	Email    string `json:"email" form:"email" validate:"required,email"`
	Password string `json:"password" form:"password" validate:"required,min=4"`
}

func TestValidateModel_ReturnsFieldErrors(t *testing.T) {
	model := basicValidation{Email: "not-an-email"}

	errs := ValidateModel(model)
	if len(errs) == 0 {
		t.Fatal("expected validation errors, got none")
	}
	if _, ok := errs["email"]; !ok {
		t.Errorf("expected an error for field 'email', got %v", errs)
	}
	if _, ok := errs["password"]; !ok {
		t.Errorf("expected an error for field 'password', got %v", errs)
	}
}

func TestValidateModel_ValidInputReturnsNoErrors(t *testing.T) {
	model := basicValidation{Email: "user@example.com", Password: "abcd"}

	errs := ValidateModel(model)
	if len(errs) != 0 {
		t.Fatalf("expected no validation errors, got %v", errs)
	}
}

func TestValidateModel_ErrorKeysUseJSONNames(t *testing.T) {
	if err := RegisterCustomValidations(); err != nil {
		t.Fatalf("failed to register custom validations: %v", err)
	}

	model := struct {
		FullName string `json:"full_name" validate:"required"`
	}{}

	errs := ValidateModel(model)
	if _, ok := errs["full_name"]; !ok {
		t.Errorf("expected error key 'full_name', got %v", errs)
	}
}

func TestRegisterCustomValidations_ReturnsNil(t *testing.T) {
	if err := RegisterCustomValidations(); err != nil {
		t.Fatalf("RegisterCustomValidations returned an error: %v", err)
	}
}

func TestRegisterCustomValidations_Reentrant(t *testing.T) {
	if err := RegisterCustomValidations(); err != nil {
		t.Fatalf("second RegisterCustomValidations call returned an error: %v", err)
	}
}

func setupValidatorTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:validator_engine_test?mode=memory&cache=shared"), &gorm.Config{
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

func TestValidateModel_IsExistsDBCustomValidator(t *testing.T) {
	origDB := database.DB
	database.DB = setupValidatorTestDB(t)
	t.Cleanup(func() { database.DB = origDB })

	if err := RegisterCustomValidations(); err != nil {
		t.Fatalf("failed to register custom validations: %v", err)
	}

	user := entities.User{
		Email:    "exists@example.com",
		Password: "hashed-password",
		Active:   boolPtr(true),
		Admin:    boolPtr(false),
	}
	if err := database.DB.Create(&user).Error; err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	type userReference struct {
		UserID uint `json:"user_id" validate:"is_exists_db=User ID"`
	}

	valid := userReference{UserID: user.ID}
	if errs := ValidateModel(valid); len(errs) != 0 {
		t.Errorf("expected no errors for existing user, got %v", errs)
	}

	missing := userReference{UserID: 999999}
	if errs := ValidateModel(missing); len(errs) == 0 {
		t.Error("expected an error for non-existent user, got none")
	}
}

func boolPtr(v bool) *bool {
	return &v
}
