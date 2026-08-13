package translationsetting

import (
	"github.com/haditssoft/haditssoft-backend/internal/shared/auth"
	"github.com/haditssoft/haditssoft-backend/internal/shared/database"
	"github.com/haditssoft/haditssoft-backend/models"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var supportedLanguages = map[string]bool{
	"Indonesia": true,
	"English":   true,
	"Urdu":      true,
	"Bengali":   true,
}

type Repository struct{}

func NewRepository() *Repository {
	return &Repository{}
}

func (r *Repository) FindByUserID(userID uint) (*models.TranslationSetting, error) {
	var model models.TranslationSetting
	result := database.DB.
		Select("language").
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
		return c.JSON(fiber.Map{"language": "Indonesia"})
	}

	return c.JSON(fiber.Map{"language": model.Language})
}

func (s *Service) Update(c *fiber.Ctx) error {
	req := new(models.TranslationSetting)
	if err := c.BodyParser(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	if req.Language == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "language is required"})
	}

	if !supportedLanguages[req.Language] {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "language is not supported"})
	}

	userID := auth.UserID(c)

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		settingModel := new(models.TranslationSetting)

		result := tx.
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("user_id = ?", userID).
			Limit(1).
			First(settingModel)

		updateNote := ""

		if result.Error != nil {
			settingModel.UserID = userID
			settingModel.Language = req.Language

			if err := tx.Create(settingModel).Error; err != nil {
				return err
			}
			updateNote = "Create translation settings"
		} else {
			settingModel.Language = req.Language
			updateNote = "Update translation settings"

			if err := tx.Save(settingModel).Error; err != nil {
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
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Internal server error", "data": err.Error()})
	}

	return c.SendStatus(fiber.StatusNoContent)
}
