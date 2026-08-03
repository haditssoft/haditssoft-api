package lastread

import (
	"github.com/haditssoft/haditssoft-backend/internal/shared/validator"
	lastReadValidations "github.com/haditssoft/haditssoft-backend/validations/lastRead"

	"github.com/gofiber/fiber/v2"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) GetOne(c *fiber.Ctx) error {
	bookName := c.Params("book_name")
	return h.svc.GetOne(c, bookName)
}

func (h *Handler) Update(c *fiber.Ctx) error {
	modelValidation := new(lastReadValidations.LastReadUpdate)
	if err := c.BodyParser(modelValidation); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": err.Error(),
		})
	}

	allErrors := validator.ValidateModel(modelValidation)
	if len(allErrors) > 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"errors": allErrors})
	}

	return h.svc.Update(c, modelValidation.BookName, modelValidation.Number)
}
