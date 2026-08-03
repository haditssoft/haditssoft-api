package font

import (
	"encoding/json"

	"github.com/haditssoft/haditssoft-backend/internal/shared/auth"
	"github.com/haditssoft/haditssoft-backend/internal/shared/database"
	"github.com/haditssoft/haditssoft-backend/models"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Setting struct {
	FontFamily string
	Fallback   string
	Weight     int
	Size       int
}

func (f Setting) MarshalJSON() ([]byte, error) {
	return json.Marshal([]interface{}{f.FontFamily, f.Fallback, f.Weight, f.Size})
}

func (f *Setting) UnmarshalJSON(data []byte) error {
	var arr []interface{}
	if err := json.Unmarshal(data, &arr); err != nil {
		return err
	}
	f.FontFamily = arr[0].(string)
	f.Fallback = arr[1].(string)
	f.Weight = int(arr[2].(float64))
	f.Size = int(arr[3].(float64))
	return nil
}

type Repository struct{}

func NewRepository() *Repository {
	return &Repository{}
}

func (r *Repository) FindByUserID(userID uint) (*models.Font, error) {
	var model models.Font
	result := database.DB.
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

	fontModel, err := s.repo.FindByUserID(userID)
	if err != nil {
		return c.JSON(fiber.Map{
			"arabic":      Setting{},
			"translation": Setting{},
		})
	}

	return c.JSON(fiber.Map{
		"arabic": Setting{
			FontFamily: fontModel.ArabicFamily,
			Fallback:   fontModel.ArabicFallback,
			Weight:     fontModel.ArabicWeight,
			Size:       fontModel.ArabicSize,
		},
		"translation": Setting{
			FontFamily: fontModel.TranslationFamily,
			Fallback:   fontModel.TranslationFallback,
			Weight:     fontModel.TranslationWeight,
			Size:       fontModel.TranslationSize,
		},
	})
}

type UpdateRequest struct {
	Arabic      Setting `json:"arabic"`
	Translation Setting `json:"translation"`
}

func (s *Service) Update(c *fiber.Ctx) error {
	var req UpdateRequest
	if err := c.BodyParser(&req); err != nil {
		return err
	}

	userID := auth.UserID(c)

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		fontModel := new(models.Font)

		result := tx.
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("user_id = ?", userID).
			Limit(1).
			First(fontModel)

		updateNote := ""

		if result.Error != nil {
			fontModel.UserID = userID
			fontModel.ArabicFamily = req.Arabic.FontFamily
			fontModel.ArabicFallback = req.Arabic.Fallback
			fontModel.ArabicWeight = req.Arabic.Weight
			fontModel.ArabicSize = req.Arabic.Size
			fontModel.TranslationFamily = req.Translation.FontFamily
			fontModel.TranslationFallback = req.Translation.Fallback
			fontModel.TranslationWeight = req.Translation.Weight
			fontModel.TranslationSize = req.Translation.Size

			if err := tx.Create(fontModel).Error; err != nil {
				return err
			}
			updateNote = "Create font settings"
		} else {
			if req.Arabic.FontFamily != "" {
				fontModel.ArabicFamily = req.Arabic.FontFamily
				fontModel.ArabicFallback = req.Arabic.Fallback
				fontModel.ArabicWeight = req.Arabic.Weight
				fontModel.ArabicSize = req.Arabic.Size
				updateNote = "Update arabic font settings"
			}
			if req.Translation.FontFamily != "" {
				fontModel.TranslationFamily = req.Translation.FontFamily
				fontModel.TranslationFallback = req.Translation.Fallback
				fontModel.TranslationWeight = req.Translation.Weight
				fontModel.TranslationSize = req.Translation.Size

				if updateNote != "" {
					updateNote = "Update font settings"
				} else {
					updateNote = "Update translation font settings"
				}
			}

			if err := tx.Save(fontModel).Error; err != nil {
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
