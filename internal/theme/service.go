package theme

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

func (r *Repository) FindByUserID(userID uint) (*models.Theme, error) {
	var model models.Theme
	result := database.DB.
		Select("theme").
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
		return c.JSON(fiber.Map{"theme": "l"})
	}

	return c.JSON(fiber.Map{"theme": model.Theme})
}

func (s *Service) Update(c *fiber.Ctx) error {
	req := new(models.Theme)
	if err := c.BodyParser(req); err != nil {
		return err
	}

	userID := auth.UserID(c)

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		themeModel := new(models.Theme)

		result := tx.
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("user_id = ?", userID).
			Limit(1).
			First(themeModel)

		updateNote := ""

		if result.Error != nil {
			themeModel.UserID = userID
			themeModel.Theme = req.Theme

			if err := tx.Create(themeModel).Error; err != nil {
				return err
			}
			updateNote = "Create theme settings"
		} else {
			themeModel.Theme = req.Theme
			updateNote = "Update theme settings"

			if err := tx.Save(themeModel).Error; err != nil {
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
