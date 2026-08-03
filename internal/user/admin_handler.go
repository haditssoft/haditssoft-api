package user

import (
	"github.com/haditssoft/haditssoft-backend/internal/shared/auth"
	"github.com/haditssoft/haditssoft-backend/internal/shared/validator"
	userValidations "github.com/haditssoft/haditssoft-backend/validations/user"
	"strconv"

	"github.com/gofiber/fiber/v2"
)

type AdminHandler struct {
	svc *Service
}

func NewAdminHandler(svc *Service) *AdminHandler {
	return &AdminHandler{svc: svc}
}

func (h *AdminHandler) GetList(c *fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "10"))
	search := c.Query("search", "")

	results, total, err := h.svc.GetList(page, limit, search)
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"data":  results,
		"total": total,
		"page":  page,
		"limit": limit,
	})
}

func (h *AdminHandler) GetOne(c *fiber.Ctx) error {
	id := c.Params("id")

	resp, err := h.svc.GetOne(id)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"message": "Not found"})
	}

	return c.JSON(resp)
}

func (h *AdminHandler) GetSome(c *fiber.Ctx) error {
	ids := c.Query("filter")

	results, err := h.svc.GetSome(ids)
	if err != nil {
		return err
	}
	if len(results) < 1 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "Not found"})
	}

	return c.JSON(results)
}

func (h *AdminHandler) Create(c *fiber.Ctx) error {
	modelValidation := new(userValidations.UserCreate)
	if err := c.BodyParser(modelValidation); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": err.Error()})
	}

	allErrors := validator.ValidateModel(modelValidation)
	if len(allErrors) > 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"errors": allErrors})
	}

	resp, err := h.svc.AdminCreate(modelValidation.Email, modelValidation.Password, modelValidation.Active, modelValidation.Admin, auth.UserID(c), c.Path(), c.IP())
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"status": "error", "message": "Internal server error", "data": err.Error()})
	}

	return c.JSON(resp)
}

func (h *AdminHandler) Update(c *fiber.Ctx) error {
	id, _ := strconv.Atoi(c.Params("id"))

	modelForm := new(userValidations.UserUpdate)
	if err := c.BodyParser(modelForm); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": err.Error()})
	}

	modelForm.ID = uint(id)
	allErrors := validator.ValidateModel(modelForm)
	if len(allErrors) > 0 {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{"errors": allErrors})
	}

	resp, err := h.svc.AdminUpdate(uint(id), modelForm.Email, modelForm.Active, modelForm.Admin, modelForm.NewPassword, auth.UserID(c), c.Path(), c.IP())
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"status": "error", "message": "Internal server error", "data": err.Error()})
	}

	return c.JSON(resp)
}

func (h *AdminHandler) DeleteOne(c *fiber.Ctx) error {
	id, _ := strconv.Atoi(c.Params("id"))

	err := h.svc.AdminDeleteOne(uint(id), auth.UserID(c), c.Path(), c.IP())
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	return c.JSON(fiber.Map{"status": "success", "message": "User deleted"})
}

func (h *AdminHandler) DeleteSome(c *fiber.Ctx) error {
	ids := c.Query("ids")

	deletedIdsStr, err := h.svc.AdminDeleteSome(ids, auth.UserID(c), c.Path(), c.IP())
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"status": "error", "message": "Internal server error", "data": err.Error()})
	}

	return c.JSON(fiber.Map{"status": "success", "message": "User(s) deleted", "data": deletedIdsStr})
}
