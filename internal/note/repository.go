package note

import (
	"time"

	"github.com/haditssoft/haditssoft-backend/internal/shared/database"
	"github.com/haditssoft/haditssoft-backend/models"

	"gorm.io/gorm"
)

type Repository struct{}

func NewRepository() *Repository {
	return &Repository{}
}

func (r *Repository) GetList(bookName string, userID uint) ([]map[string]interface{}, error) {
	container := []map[string]interface{}{}
	result := database.DB.Table(bookName+"Note").
		Select("*").
		Where("user_id = ?", userID).
		Where("deleted_at IS NULL").
		Find(&container)
	return container, result.Error
}

func (r *Repository) GetOne(bookName string, hadithID uint, userID uint) (map[string]interface{}, error) {
	container := map[string]interface{}{}
	result := database.DB.Table(bookName+"Note").
		Select("*").
		Where("hadith_id = ?", hadithID).
		Where("user_id = ?", userID).
		Where("deleted_at IS NULL").
		Take(&container)
	return container, result.Error
}

func (r *Repository) Create(bookName string, hadithID uint, note string, userID uint, path, ip string) error {
	return database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Table(bookName + "Note").Create(map[string]interface{}{
			"hadith_id":  hadithID,
			"note":       note,
			"user_id":    userID,
			"created_at": time.Now(),
			"updated_at": time.Now(),
		}).Error; err != nil {
			return err
		}

		activity := models.Activity{
			ActionURL: path,
			ReqMethod: "POST",
			Note:      "Create new note",
			IP:        ip,
			UserID:    userID,
		}
		if err := tx.Create(&activity).Error; err != nil {
			return err
		}

		return nil
	})
}

func (r *Repository) Update(bookName string, id uint, hadithID uint, note string, userID uint, path, ip string) error {
	return database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Table(bookName+"Note").Where("id = ?", id).Updates(map[string]interface{}{
			"hadith_id":  hadithID,
			"note":       note,
			"updated_at": time.Now(),
		}).Error; err != nil {
			return err
		}

		activity := models.Activity{
			ActionURL: path,
			ReqMethod: "PUT",
			Note:      "Update note",
			IP:        ip,
			UserID:    userID,
		}
		if err := tx.Create(&activity).Error; err != nil {
			return err
		}

		return nil
	})
}

func (r *Repository) FindNoteIDByHadithAndUser(bookName string, hadithID uint, userID uint) (uint, error) {
	var noteID uint
	result := database.DB.Table(bookName+"Note").
		Select("id").
		Where("hadith_id = ?", hadithID).
		Where("user_id = ?", userID).
		Where("deleted_at IS NULL").
		Limit(1).
		Scan(&noteID)
	return noteID, result.Error
}

func (r *Repository) DeleteOne(bookName string, hadithID uint, userID uint, path, ip string) error {
	return database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Table(bookName+"Note").
			Where("hadith_id = ?", hadithID).
			Where("user_id = ?", userID).
			Where("deleted_at IS NULL").
			Updates(map[string]interface{}{
				"deleted_at": time.Now(),
			}).Error; err != nil {
			return err
		}

		activity := models.Activity{
			ActionURL: path,
			ReqMethod: "DELETE",
			Note:      "Delete note",
			IP:        ip,
			UserID:    userID,
		}
		if err := tx.Create(&activity).Error; err != nil {
			return err
		}

		return nil
	})
}
