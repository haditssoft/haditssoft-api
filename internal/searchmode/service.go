package searchmode

import (
	"github.com/haditssoft/haditssoft-backend/internal/shared/auth"
	"github.com/haditssoft/haditssoft-backend/internal/shared/database"
	"github.com/haditssoft/haditssoft-backend/models"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository struct{}

func NewRepository() *Repository {
	return &Repository{}
}

func (r *Repository) FindByUserID(userID uint) (*models.SearchMode, error) {
	var model models.SearchMode
	result := database.DB.
		Select("search_mode").
		Where("user_id = ?", userID).
		Limit(1).
		First(&model)
	return &model, result.Error
}

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) GetOne(c *fiber.Ctx) error {
	userID := auth.UserID(c)

	model, err := s.repo.FindByUserID(userID)
	if err != nil {
		return c.JSON(fiber.Map{"search_mode": 1})
	}

	return c.JSON(fiber.Map{"search_mode": model.SearchMode})
}

func (s *Service) Update(c *fiber.Ctx) error {
	req := new(models.SearchMode)
	if err := c.BodyParser(req); err != nil {
		return err
	}

	userID := auth.UserID(c)

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		model := new(models.SearchMode)

		result := tx.
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("user_id = ?", userID).
			Limit(1).
			First(model)

		updateNote := ""

		if result.Error != nil {
			model.UserID = userID
			model.SearchMode = req.SearchMode

			if err := tx.Create(model).Error; err != nil {
				return err
			}
			updateNote = "Create theme settings"
		} else {
			model.SearchMode = req.SearchMode
			updateNote = "Update theme settings"

			if err := tx.Save(model).Error; err != nil {
				return err
			}
		}

		activity := models.Activity{
			ActionURL: c.Path(),
			ReqMethod: c.Method(),
			Note:      updateNote,
			IP:        c.IP(),
			UserID:    userID,
		}
		if err := tx.Create(&activity).Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return c.Status(500).JSON(fiber.Map{"status": "error", "message": "Internal server error", "data": err.Error()})
	}

	return c.SendStatus(fiber.StatusNoContent)
}
