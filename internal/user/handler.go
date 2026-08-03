package user

import (
	"strconv"

	"github.com/haditssoft/haditssoft-backend/internal/shared/auth"
	"github.com/haditssoft/haditssoft-backend/internal/shared/validator"
	userValidations "github.com/haditssoft/haditssoft-backend/validations/user"

	"github.com/gofiber/fiber/v2"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) GetOne(c *fiber.Ctx) error {
	id := c.Params("id")

	resp, err := h.svc.GetOne(id)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"message": "Not found"})
	}

	return c.JSON(resp)
}

func (h *Handler) Create(c *fiber.Ctx) error {
	modelValidation := new(userValidations.UserCreate)
	if err := c.BodyParser(modelValidation); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": err.Error()})
	}

	modelValidation.Active = "false"
	modelValidation.Admin = "false"
	allErrors := validator.ValidateModel(modelValidation)
	if len(allErrors) > 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"errors": allErrors})
	}

	token, err := h.svc.Create(modelValidation.Email, modelValidation.Password, modelValidation.PasswordConfirmation, c.Path(), c.IP())
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"status": "error", "message": "Internal server error", "data": err.Error()})
	}

	return c.JSON(fiber.Map{"status": "success", "message": "Success create user", "token": token})
}

func (h *Handler) Verify(c *fiber.Ctx) error {
	modelValidation := new(userValidations.UserVerify)
	if err := c.BodyParser(modelValidation); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": err.Error()})
	}

	allErrors := validator.ValidateModel(modelValidation)
	if len(allErrors) > 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"errors": allErrors})
	}

	userID := auth.UserID(c)
	err := h.svc.Verify(userID, modelValidation.Code, c.Path(), c.IP())
	if err != nil {
		switch err.Error() {
		case "User not found":
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
		case "Email already verified", "Invalid verification code", "Verification code expired":
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
		default:
			return c.Status(500).JSON(fiber.Map{"status": "error", "message": "Internal server error", "data": err.Error()})
		}
	}

	return c.JSON(fiber.Map{"status": "success", "message": "Email verified successfully"})
}

func (h *Handler) Resend(c *fiber.Ctx) error {
	userID := auth.UserID(c)

	err := h.svc.Resend(userID)
	if err != nil {
		switch err.Error() {
		case "User not found":
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
		case "Email already verified", "Please wait before requesting a new code":
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
		default:
			return c.Status(500).JSON(fiber.Map{"status": "error", "message": "Internal server error", "data": err.Error()})
		}
	}

	return c.JSON(fiber.Map{"status": "success", "message": "Verification code resent successfully"})
}

func (h *Handler) Update(c *fiber.Ctx) error {
	id, _ := strconv.Atoi(c.Params("id"))

	if uint(id) != auth.UserID(c) {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "Forbidden", "data": nil})
	}

	modelForm := new(userValidations.UserUpdate)
	if err := c.BodyParser(modelForm); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": err.Error()})
	}

	modelForm.ID = uint(id)
	allErrors := validator.ValidateModel(modelForm)
	if len(allErrors) > 0 {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{"errors": allErrors})
	}

	resp, err := h.svc.Update(uint(id), auth.UserID(c), modelForm.Email, modelForm.Active, modelForm.Admin, modelForm.NewPassword, c.Path(), c.IP())
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"status": "error", "message": "Internal server error", "data": err.Error()})
	}

	return c.JSON(resp)
}

func (h *Handler) DeleteOne(c *fiber.Ctx) error {
	id, _ := strconv.Atoi(c.Params("id"))

	err := h.svc.DeleteOne(uint(id), auth.UserID(c), c.Path(), c.IP())
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	return c.JSON(fiber.Map{"status": "success", "message": "User deleted"})
}

func (h *Handler) ForgotPassword(c *fiber.Ctx) error {
	modelValidation := new(userValidations.ForgotPasswordRequest)
	if err := c.BodyParser(modelValidation); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": err.Error()})
	}

	allErrors := validator.ValidateModel(modelValidation)
	if len(allErrors) > 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"errors": allErrors})
	}

	err := h.svc.ForgotPassword(modelValidation.Email, c.Path(), c.IP())
	if err != nil {
		if err.Error() == "Please wait before requesting a new code" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
		}
		return c.Status(500).JSON(fiber.Map{"status": "error", "message": "Internal server error", "data": err.Error()})
	}

	return c.JSON(fiber.Map{"status": "success", "message": "If an account with that email exists, a verification code has been sent"})
}

func (h *Handler) ConfirmForgotPassword(c *fiber.Ctx) error {
	modelValidation := new(userValidations.ForgotPasswordConfirm)
	if err := c.BodyParser(modelValidation); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": err.Error()})
	}

	allErrors := validator.ValidateModel(modelValidation)
	if len(allErrors) > 0 {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{"errors": allErrors})
	}

	err := h.svc.ConfirmForgotPassword(modelValidation.Email, modelValidation.Code, modelValidation.NewPassword, c.Path(), c.IP())
	if err != nil {
		switch err.Error() {
		case "User not found", "record not found":
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "User not found", "data": nil})
		case "Invalid verification code", "Verification code expired":
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
		default:
			return c.Status(500).JSON(fiber.Map{"status": "error", "message": "Internal server error", "data": err.Error()})
		}
	}

	return c.JSON(fiber.Map{"status": "success", "message": "Password has been reset successfully"})
}
