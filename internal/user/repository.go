package user

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/haditssoft/haditssoft-backend/internal/shared/database"
	"github.com/haditssoft/haditssoft-backend/internal/shared/email"
	"github.com/haditssoft/haditssoft-backend/internal/shared/utils"
	"github.com/haditssoft/haditssoft-backend/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository struct{}

func NewRepository() *Repository {
	return &Repository{}
}

func (r *Repository) DB() *gorm.DB {
	return database.DB
}

func (r *Repository) Transaction(fn func(tx *gorm.DB) error) error {
	return database.DB.Transaction(fn)
}

func (r *Repository) TransactionWithContext(ctx context.Context, fn func(tx *gorm.DB) error) error {
	return database.DB.WithContext(ctx).Transaction(fn)
}

func (r *Repository) GetByID(id interface{}) (*models.User, error) {
	var model models.User
	result := database.DB.First(&model, "id = ?", id)
	if result.Error != nil {
		return nil, result.Error
	}
	return &model, nil
}

func (r *Repository) GetByEmail(email string) (*models.User, error) {
	var model models.User
	result := database.DB.Where("email = ?", email).First(&model)
	if result.Error != nil {
		return nil, result.Error
	}
	return &model, nil
}

func (r *Repository) GetByEmailOrNotFound(email string) (*models.User, error) {
	var model models.User
	result := database.DB.Where("email = ?", email).First(&model)
	if result.Error != nil {
		return nil, result.Error
	}
	return &model, nil
}

func (r *Repository) Save(model *models.User) error {
	return database.DB.Save(model).Error
}

func (r *Repository) SaveTx(tx *gorm.DB, model *models.User) error {
	return tx.Save(model).Error
}

func (r *Repository) CreateTx(tx *gorm.DB, model *models.User) error {
	return tx.Create(model).Error
}

func (r *Repository) CreateWithSkipHooks(model *models.User) error {
	return database.DB.Session(&gorm.Session{SkipHooks: true}).Create(model).Error
}

func (r *Repository) CreateActivityTx(tx *gorm.DB, activity *models.Activity) error {
	return tx.Create(activity).Error
}

func (r *Repository) FirstWithLockTx(tx *gorm.DB, dest *models.User, id interface{}) error {
	return tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(dest, "id = ?", id).Error
}

func (r *Repository) FirstByID(dest *models.User, id interface{}) error {
	return database.DB.Where("id = ?", id).First(dest).Error
}

func (r *Repository) DeleteTx(tx *gorm.DB, model *models.User) error {
	return tx.Delete(model).Error
}

func (r *Repository) FindUsersByIDs(ids []string) ([]models.User, error) {
	var model []models.User
	result := database.DB.Find(&model, ids)
	if result.Error != nil {
		return nil, result.Error
	}
	return model, nil
}

func (r *Repository) GetSomeUsers(ids []string) ([]responseField, error) {
	var results []responseField
	result := database.DB.Model(&models.User{}).Select(
		"id", "email", "active", "admin",
		"created_by", "updated_by", "created_at", "updated_at",
	).Where("id IN (?)", ids).Find(&results)
	if result.Error != nil {
		return nil, result.Error
	}
	return results, nil
}

type responseField struct {
	ID        uint      `json:"id"`
	Email     string    `json:"email"`
	Active    *bool     `json:"active"`
	Admin     *bool     `json:"admin"`
	CreatedBy string    `json:"created_by"`
	UpdatedBy string    `json:"updated_by"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (r *Repository) SearchUser(page, limit int, search string) ([]models.User, int64, error) {
	return models.SearchUser(page, limit, search)
}

func (r *Repository) GenerateVerificationCode() string {
	return utils.GenerateVerificationCode()
}

func (r *Repository) SendVerificationCode(emails []string, to, code string) {
	go email.SendVerificationCode(emails, to, code)
}

func (r *Repository) SendConfirmation(emails []string, subject string) {
	go email.Sender(emails, subject)
}

func (r *Repository) ParseJSONMapIDs(raw string) ([]string, error) {
	var parsed map[string][]string
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return nil, err
	}
	return parsed["id"], nil
}

func (r *Repository) ParseJSONArray(raw string) ([]string, error) {
	var parsed []string
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return nil, err
	}
	return parsed, nil
}

func (r *Repository) NewUserWithCode(email, password string) (*models.User, string) {
	code := utils.GenerateVerificationCode()
	user := &models.User{
		Email:                     email,
		Password:                  password,
		VerificationCode:          code,
		VerificationCodeExpiresAt: sql.NullTime{Time: time.Now().Add(15 * time.Minute), Valid: true},
	}
	return user, code
}
