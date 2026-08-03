package bookmark

import (
	"github.com/haditssoft/haditssoft-backend/internal/shared/auth"
	"github.com/haditssoft/haditssoft-backend/internal/shared/validator"
	bookmarkValidations "github.com/haditssoft/haditssoft-backend/validations/bookmark"

	"github.com/gofiber/fiber/v2"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) GetList(c *fiber.Ctx) error {
	userID := auth.UserID(c)

	titles, err := h.svc.GetList(userID)
	if err != nil {
		if err.Error() == "Bookmark not found" {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"message": err.Error()})
		}
		return err
	}

	return c.JSON(titles)
}

func (h *Handler) GetOne(c *fiber.Ctx) error {
	title := c.Params("title")
	userID := auth.UserID(c)

	resp, err := h.svc.GetOne(userID, title)
	if err != nil {
		if err.Error() == "title parameter is required" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": err.Error()})
		}
		return err
	}

	return c.JSON(resp)
}

func (h *Handler) GetSome(c *fiber.Ctx) error {
	title := c.Params("title")
	bookName := c.Params("book_name")
	userID := auth.UserID(c)

	resp, err := h.svc.GetSome(userID, title, bookName)
	if err != nil {
		switch err.Error() {
		case "title parameter is required", "book name parameter is required":
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": err.Error()})
		default:
			return err
		}
	}

	return c.JSON(resp)
}

func (h *Handler) Create(c *fiber.Ctx) error {
	userID := auth.UserID(c)

	modelValidation := new(bookmarkValidations.BookmarkCreate)
	modelValidation.UserID = userID

	if err := c.BodyParser(modelValidation); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": err.Error()})
	}

	allErrors := validator.ValidateModel(modelValidation)
	if len(allErrors) > 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"errors": allErrors})
	}

	err := h.svc.Create(userID, modelValidation.Title, modelValidation.Items.BookName, modelValidation.Items.BookNumber, c.Path(), c.IP())
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"status": "error", "message": "Internal server error", "data": err.Error()})
	}

	return c.SendStatus(fiber.StatusNoContent)
}

func (h *Handler) UpdateAll(c *fiber.Ctx) error {
	title := c.Params("title")
	bookName := c.Params("book_name")
	userID := auth.UserID(c)

	var payload []uint
	if err := c.BodyParser(&payload); err != nil {
		return err
	}

	err := h.svc.UpdateAll(userID, title, bookName, payload, c.Path(), c.IP())
	if err != nil {
		if err.Error() == "payload is required" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": err.Error()})
		}
		if err.Error() == "not found" {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"message": "Bookmark not found"})
		}
		return c.Status(500).JSON(fiber.Map{"status": "error", "message": "Internal server error", "data": err.Error()})
	}

	return c.SendStatus(fiber.StatusNoContent)
}

func (h *Handler) DeleteParent(c *fiber.Ctx) error {
	title := c.Params("title")
	bookName := c.Params("book_name")
	userID := auth.UserID(c)

	rowsAffected, err := h.svc.DeleteParent(userID, title, bookName, c.Path(), c.IP())
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"status": "error", "message": "Internal server error", "data": err.Error()})
	}

	return c.JSON(fiber.Map{"status": "success", "message": "User deleted", "data": rowsAffected})
}
