package note

import (
	"github.com/haditssoft/haditssoft-backend/internal/shared/auth"
	noteValidations "github.com/haditssoft/haditssoft-backend/validations/note"
	"strconv"

	"github.com/haditssoft/haditssoft-backend/internal/shared/validator"

	"github.com/gofiber/fiber/v2"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) GetList(c *fiber.Ctx) error {
	bookName := normalizeBookName(c.Params("book_name"))
	userID := auth.UserID(c)
	return h.svc.GetList(c, bookName, userID)
}

func (h *Handler) GetOne(c *fiber.Ctx) error {
	bookName := normalizeBookName(c.Params("book_name"))
	hadithIDStr := c.Params("hadith_id")
	hadithIDUint, err := strconv.ParseUint(hadithIDStr, 10, 32)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Invalid ID format",
		})
	}
	userID := auth.UserID(c)
	return h.svc.GetOne(c, bookName, uint(hadithIDUint), userID)
}

func (h *Handler) Create(c *fiber.Ctx) error {
	hadithIDStr := c.Params("hadith_id")
	bookName := normalizeBookName(c.Params("book_name"))

	hadithIDUint, err := strconv.ParseUint(hadithIDStr, 10, 32)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Invalid ID format",
		})
	}

	modelValidation := new(noteValidations.NoteCreate)
	if err := c.BodyParser(modelValidation); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": err.Error(),
		})
	}
	modelValidation.HadithID = uint(hadithIDUint)
	modelValidation.BookName = bookName

	allErrors := validator.ValidateModel(modelValidation)
	if len(allErrors) > 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"errors": allErrors})
	}

	userID := auth.UserID(c)
	return h.svc.Create(c, bookName, uint(hadithIDUint), modelValidation.Note, userID)
}

func (h *Handler) Update(c *fiber.Ctx) error {
	hadithIDStr := c.Params("hadith_id")
	bookName := normalizeBookName(c.Params("book_name"))

	hadithIDUint, err := strconv.ParseUint(hadithIDStr, 10, 32)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Invalid ID format",
		})
	}

	modelValidation := new(noteValidations.NoteUpdate)
	if err := c.BodyParser(modelValidation); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": err.Error(),
		})
	}

	modelValidation.HadithID = uint(hadithIDUint)
	modelValidation.BookName = bookName

	userID := auth.UserID(c)

	noteID, err := h.svc.FindNoteID(bookName, uint(hadithIDUint), userID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"message": "Note not found"})
	}
	modelValidation.ID = noteID

	allErrors := validator.ValidateModel(modelValidation)
	if len(allErrors) > 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"errors": allErrors})
	}

	return h.svc.Update(c, modelValidation.ID, modelValidation.HadithID, modelValidation.Note, modelValidation.BookName, userID, c.Path(), c.IP())
}

func (h *Handler) DeleteOne(c *fiber.Ctx) error {
	hadithIDStr := c.Params("hadith_id")
	bookName := normalizeBookName(c.Params("book_name"))

	hadithIDUint, err := strconv.ParseUint(hadithIDStr, 10, 32)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Invalid ID format",
		})
	}

	userID := auth.UserID(c)
	return h.svc.DeleteOne(c, bookName, uint(hadithIDUint), userID, c.Path(), c.IP())
}

func (h *Handler) ValidateDelete(c *fiber.Ctx) error {
	hadithIDStr := c.Params("hadith_id")
	bookName := normalizeBookName(c.Params("book_name"))

	hadithIDUint, err := strconv.ParseUint(hadithIDStr, 10, 32)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Invalid ID format",
		})
	}

	modelValidation := &noteValidations.NoteDelete{
		HadithID: uint(hadithIDUint),
		BookName: bookName,
	}

	allErrors := validator.ValidateModel(modelValidation)
	if len(allErrors) > 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"errors": allErrors})
	}

	return c.JSON(fiber.Map{"status": "success", "note": "Valid to delete"})
}
