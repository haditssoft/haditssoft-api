package note

import (
	"strconv"
	"time"

	"github.com/haditssoft/haditssoft-backend/internal/shared/database"
	"github.com/haditssoft/haditssoft-backend/models"

	"gorm.io/gorm"
)

type Repository struct{}

func NewRepository() *Repository {
	return &Repository{}
}

var noteTablePrefixes = map[string]string{
	"bukharinote":       "ShahihBukhari",
	"muslimnote":        "ShahihMuslim",
	"tirmidzinote":      "SunanTirmidzi",
	"abudaudnote":       "SunanAbuDaud",
	"nasainote":         "SunanNasai",
	"ibnumajahnote":     "SunanIbnuMajah",
	"dariminote":        "SunanDarimi",
	"ahmadnote":         "MusnadAhmad",
	"maliknote":         "MuwathaMalik",
	"daruquthninote":    "SunanDaruquthni",
	"ibnukhuzaimahnote": "ShahihIbnuKhuzaimah",
	"ibnuhibbannote":    "ShahihIbnuHibban",
	"mustadraknote":     "AlMustadrak",
	"syafiinote":        "MusnadSyafii",
}

func normalizeBookName(bookName string) string {
	if prefix, ok := noteTablePrefixes[bookName]; ok {
		return prefix
	}
	return bookName
}

func noteTableName(bookName string) string {
	return normalizeBookName(bookName) + "Note"
}

func (r *Repository) GetList(bookName string, userID uint) (map[string]string, error) {
	type noteRow struct {
		HadithID uint   `gorm:"column:hadith_id"`
		Note     string `gorm:"column:note"`
	}

	rows := []noteRow{}
	result := database.DB.Table(noteTableName(bookName)).
		Select("hadith_id, note").
		Where("user_id = ?", userID).
		Where("deleted_at IS NULL").
		Find(&rows)
	if result.Error != nil {
		return nil, result.Error
	}

	notes := make(map[string]string, len(rows))
	for _, row := range rows {
		notes[strconv.FormatUint(uint64(row.HadithID), 10)] = row.Note
	}
	return notes, nil
}

func (r *Repository) GetOne(bookName string, hadithID uint, userID uint) (map[string]interface{}, error) {
	container := map[string]interface{}{}
	result := database.DB.Table(noteTableName(bookName)).
		Select("*").
		Where("hadith_id = ?", hadithID).
		Where("user_id = ?", userID).
		Where("deleted_at IS NULL").
		Take(&container)
	return container, result.Error
}

func (r *Repository) Create(bookName string, hadithID uint, note string, userID uint, path, ip string) error {
	return database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Table(noteTableName(bookName)).Create(map[string]interface{}{
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
		if err := tx.Table(noteTableName(bookName)).Where("id = ?", id).Updates(map[string]interface{}{
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
	result := database.DB.Table(noteTableName(bookName)).
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
		if err := tx.Table(noteTableName(bookName)).
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
