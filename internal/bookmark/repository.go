package bookmark

import (
	"time"

	"github.com/haditssoft/haditssoft-backend/internal/shared/database"
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

func (r *Repository) GetTitlesByUserID(userID uint) ([]string, error) {
	var titles []string
	result := database.DB.Table("bookmark").
		Where("user_id = ?", userID).
		Where("deleted_at IS NULL").
		Order("title ASC").
		Pluck("title", &titles)
	if result.Error != nil {
		return nil, result.Error
	}
	return titles, nil
}

type itemRow struct {
	BookName   string `gorm:"column:book_name"`
	BookNumber int    `gorm:"column:book_number"`
}

func (r *Repository) GetItemsByTitle(userID uint, title string) ([]itemRow, error) {
	var items []itemRow
	result := database.DB.Table("bookmark").
		Select("bookmark_item.book_name", "bookmark_item.book_number").
		Joins("LEFT JOIN bookmark_item ON bookmark_item.bookmark_id = bookmark.id AND bookmark_item.deleted_at IS NULL").
		Where("bookmark.user_id = ?", userID).
		Where("bookmark.title = ?", title).
		Where("bookmark.deleted_at IS NULL").
		Order("bookmark_item.book_name ASC").
		Order("bookmark_item.book_number ASC").
		Find(&items)
	if result.Error != nil {
		return nil, result.Error
	}
	return items, nil
}

func (r *Repository) GetBookNumbersByTitleAndBookName(userID uint, title, bookName string) ([]int64, error) {
	var bookNumbers []int64
	result := database.DB.Table("bookmark").
		Joins("INNER JOIN bookmark_item ON bookmark_item.bookmark_id = bookmark.id AND bookmark_item.deleted_at IS NULL").
		Where("bookmark.user_id = ?", userID).
		Where("bookmark.title = ?", title).
		Where("bookmark_item.book_name = ?", bookName).
		Where("bookmark.deleted_at IS NULL").
		Order("bookmark_item.book_number ASC").
		Pluck("bookmark_item.book_number", &bookNumbers)
	if result.Error != nil {
		return nil, result.Error
	}
	return bookNumbers, nil
}

func (r *Repository) FindBookmarkIDByUserAndTitle(userID uint, title string) uint {
	var bookmarkID uint
	database.DB.Table("bookmark").
		Select("id").
		Where("user_id = ?", userID).
		Where("title = ?", title).
		Where("deleted_at IS NULL").
		Limit(1).
		Scan(&bookmarkID)
	return bookmarkID
}

func (r *Repository) CreateBookmarkTx(tx *gorm.DB, userID uint, title string, now time.Time) (uint, error) {
	type bookmarkRow struct {
		ID        uint      `gorm:"column:id;primaryKey;autoIncrement"`
		UserID    uint      `gorm:"column:user_id"`
		Title     string    `gorm:"column:title"`
		CreatedAt time.Time `gorm:"column:created_at"`
		UpdatedAt time.Time `gorm:"column:updated_at"`
	}

	bookmark := bookmarkRow{
		UserID:    userID,
		Title:     title,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := tx.Table("bookmark").Create(&bookmark).Error; err != nil {
		return 0, err
	}
	return bookmark.ID, nil
}

func (r *Repository) CreateBookmarkItemTx(tx *gorm.DB, bookmarkID uint, bookName string, bookNumber uint, now time.Time) error {
	return tx.Table("bookmark_item").Create(map[string]interface{}{
		"bookmark_id": bookmarkID,
		"book_name":   bookName,
		"book_number": bookNumber,
		"created_at":  now,
	}).Error
}

func (r *Repository) CreateActivityTx(tx *gorm.DB, userID uint, path, method, note, ip string) error {
	activity := models.Activity{
		ActionURL: path,
		ReqMethod: method,
		Note:      note,
		IP:        ip,
		UserID:    userID,
	}
	return tx.Create(&activity).Error
}

func (r *Repository) FindBookmarkItemToDeleteTx(tx *gorm.DB, userID uint, title, bookName string, payload []uint) (uint, error) {
	var bookmarkItemID int

	result := tx.Table("bookmark_item").
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Select("bookmark_item.id").
		Joins("INNER JOIN bookmark ON bookmark.id = bookmark_item.bookmark_id").
		Where("bookmark.user_id = ?", userID).
		Where("bookmark.title = ?", title).
		Where("bookmark.deleted_at IS NULL").
		Where("bookmark_item.book_name = ?", bookName).
		Where("bookmark_item.book_number NOT IN (?)", payload).
		Where("bookmark_item.deleted_at IS NULL").
		Limit(1).
		Scan(&bookmarkItemID)

	if result.Error != nil {
		return 0, result.Error
	}
	return uint(bookmarkItemID), nil
}

func (r *Repository) SoftDeleteBookmarkItemTx(tx *gorm.DB, itemID uint) error {
	now := time.Now()
	return tx.Table("bookmark_item").
		Where("bookmark_item.id = ?", itemID).
		Update("deleted_at", now).Error
}

func (r *Repository) FindBookmarkItemForDeleteTx(tx *gorm.DB, userID uint, title, bookName string) (*models.BookmarkItem, error) {
	var model models.BookmarkItem
	result := tx.
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Joins("Bookmark").
		Where("book_name = ?", bookName).
		Where("Bookmark.user_id = ?", userID).
		Where("Bookmark.title = ?", title).
		First(&model)
	if result.Error != nil {
		return nil, result.Error
	}
	return &model, nil
}

func (r *Repository) SoftDeleteModelTx(tx *gorm.DB, model *models.BookmarkItem) error {
	return tx.Delete(model).Error
}
