package lastread

import (
	"time"

	"github.com/haditssoft/haditssoft-backend/internal/shared/auth"
	"github.com/haditssoft/haditssoft-backend/internal/shared/database"
	"github.com/haditssoft/haditssoft-backend/models"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type Repository struct{}

func NewRepository() *Repository {
	return &Repository{}
}

func (r *Repository) GetOne(userID uint, bookName string) (map[string]interface{}, error) {
	container := map[string]interface{}{}
	result := database.DB.Table("LastRead").
		Select("no AS number").
		Where("user_id = ?", userID).
		Where("book_name = ?", bookName).
		Take(&container)
	return container, result.Error
}

func (r *Repository) UpdateOrCreate(userID uint, bookName string, number uint) error {
	return database.DB.Transaction(func(tx *gorm.DB) error {
		result := tx.
			Table("LastRead").
			Where("user_id = ?", userID).
			Where("book_name = ?", bookName).
			Updates(map[string]interface{}{
				"no":         number,
				"updated_at": time.Now(),
			})

		if result.Error != nil {
			return result.Error
		}

		if result.RowsAffected == 0 {
			if err := tx.Table("LastRead").Create(map[string]interface{}{
				"user_id":    userID,
				"book_name":  bookName,
				"no":         number,
				"updated_at": time.Now(),
			}).Error; err != nil {
				return err
			}
		}

		return nil
	})
}

func (r *Repository) CreateActivity(userID uint, path, method, ip string) error {
	activity := models.Activity{
		ActionURL: path,
		ReqMethod: method,
		Note:      "Update last read",
		IP:        ip,
		UserID:    userID,
	}
	return database.DB.Create(&activity).Error
}

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) GetOne(c *fiber.Ctx, bookName string) error {
	if bookName == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "book_name parameter is required",
		})
	}

	userID := auth.UserID(c)

	container, err := s.repo.GetOne(userID, bookName)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"message": "Note not found",
			})
		}
		return err
	}

	return c.JSON(container)
}

func (s *Service) Update(c *fiber.Ctx, bookName string, number uint) error {
	userID := auth.UserID(c)

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := s.repo.UpdateOrCreate(userID, bookName, number); err != nil {
			return err
		}

		if err := s.repo.CreateActivity(userID, c.Path(), c.Method(), c.IP()); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return c.Status(500).JSON(fiber.Map{"status": "error", "message": "Internal server error", "data": err.Error()})
	}

	return c.SendStatus(fiber.StatusNoContent)
}
