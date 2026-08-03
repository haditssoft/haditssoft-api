package auth

import (
	"github.com/haditssoft/haditssoft-backend/internal/shared/database"
	"github.com/haditssoft/haditssoft-backend/internal/shared/database/entities"

	"gorm.io/gorm"
)

type Repository struct{}

func NewRepository() *Repository {
	return &Repository{}
}

func (r *Repository) GetUserByEmail(email string) (*entities.User, error) {
	var user entities.User
	if err := database.DB.Select("ID", "Email", "Password").Where("email = ?", email).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *Repository) GetActiveAdminByEmail(email string) (*entities.User, error) {
	var user entities.User
	actv := true
	if err := database.DB.Select("ID", "Email", "Password").Where(&entities.User{Email: email, Active: &actv, Admin: &actv}).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *Repository) FindRefreshTokenByHash(hash string) (*entities.RefreshToken, error) {
	var record entities.RefreshToken
	result := database.DB.Where("token_hash = ?", hash).First(&record)
	if result.Error != nil {
		return nil, result.Error
	}
	return &record, nil
}

func (r *Repository) CreateRefreshToken(token *entities.RefreshToken) error {
	return database.DB.Create(token).Error
}

func (r *Repository) MarkTokenUsed(token *entities.RefreshToken) error {
	return database.DB.Model(token).Update("is_used", true).Error
}

func (r *Repository) RevokeAllRefreshTokens(userID uint) error {
	return database.DB.Model(&entities.RefreshToken{}).
		Where("user_id = ? AND is_used = ?", userID, false).
		Update("is_used", true).Error
}

func (r *Repository) CreateBlacklistToken(token string) error {
	model := entities.BlacklistToken{Token: token}
	return database.DB.Create(&model).Error
}

func (r *Repository) IsTokenBlacklisted(token string) (bool, error) {
	var model entities.BlacklistToken
	result := database.DB.Select("id").Where("token = ?", token).Find(&model).Limit(1)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected != 0, nil
}

func (r *Repository) IsUserActive(userID uint) (bool, error) {
	var usr entities.User
	result := database.DB.Select("id").Unscoped().Where("id = ?", userID).Where("active", false).Find(&usr).Limit(1)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected == 0, nil
}

func (r *Repository) UserExists(userID uint) (bool, error) {
	var usr entities.User
	result := database.DB.Select("id").Unscoped().Where("id = ?", userID).Find(&usr).Limit(1)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

func (r *Repository) IsUserDeleted(userID uint) (bool, error) {
	var usr entities.User
	result := database.DB.Select("id").Unscoped().Where("id = ?", userID).Where("deletedAt IS NOT NULL").Find(&usr).Limit(1)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

func (r *Repository) GetUserByID(userID uint) (*entities.User, error) {
	var user entities.User
	if err := database.DB.Select("id", "email").First(&user, userID).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *Repository) CreateActivity(userID uint, actionURL, reqMethod, note, ip string) error {
	activity := entities.Activity{
		ActionURL: actionURL,
		ReqMethod: reqMethod,
		Note:      note,
		IP:        ip,
		UserID:    userID,
	}
	return database.DB.Create(&activity).Error
}

func (r *Repository) CreateActivityTx(tx *gorm.DB, userID uint, actionURL, reqMethod, note, ip string) error {
	activity := entities.Activity{
		ActionURL: actionURL,
		ReqMethod: reqMethod,
		Note:      note,
		IP:        ip,
		UserID:    userID,
	}
	return tx.Create(&activity).Error
}

func (r *Repository) GetUserFromRefreshToken(record *entities.RefreshToken) (*entities.User, error) {
	var user entities.User
	if err := database.DB.Select("id", "email").First(&user, record.UserID).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *Repository) ExpireRefreshToken(record *entities.RefreshToken) error {
	return database.DB.Model(record).Update("is_used", true).Error
}

func (r *Repository) BlacklistTokenIfExists(token string, userID uint, exists, deleted bool) error {
	if !exists || deleted {
		blModel := entities.BlacklistToken{Token: token}
		return database.DB.Create(&blModel).Error
	}
	return nil
}
