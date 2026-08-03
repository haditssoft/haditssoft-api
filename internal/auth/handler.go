package auth

import (
	"net/mail"

	"github.com/haditssoft/haditssoft-backend/internal/shared/auth"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v4"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Login(c *fiber.Ctx) error {
	input := new(LoginInput)
	if err := c.BodyParser(input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "Error on login request", "data": err})
	}

	if _, err := mail.ParseAddress(input.Email); err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"status": "error", "message": "Invalid email format"})
	}

	resp, err := h.svc.Login(input.Email, input.Password, c.IP(), c.Path())
	if err != nil {
		status := fiber.StatusUnauthorized
		if err.Error() == "failed to generate access token" || err.Error() == "failed to generate refresh token" || err.Error() == "failed to create refresh token" || err.Error() == "failed to log activity" {
			status = fiber.StatusInternalServerError
		}
		return c.Status(status).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	return c.JSON(resp)
}

func (h *Handler) Logout(c *fiber.Ctx) error {
	token := c.Locals("user").(*jwt.Token)
	uid := auth.UserID(c)

	if err := h.svc.Logout(uid, token.Raw, c.IP(), c.Path()); err != nil {
		return c.Status(500).JSON(fiber.Map{"status": "error", "message": "Internal server error", "data": err.Error()})
	}

	return c.JSON(fiber.Map{"status": "success", "message": "Your are logged out now", "data": nil})
}

func (h *Handler) Identity(c *fiber.Ctx) error {
	uid := auth.UserID(c)

	resp, err := h.svc.Identity(uid)
	if err != nil {
		return err
	}

	return c.JSON(resp)
}

func (h *Handler) Refresh(c *fiber.Ctx) error {
	input := new(RefreshInput)
	if err := c.BodyParser(input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "Invalid request body"})
	}

	if input.RefreshToken == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "refresh_token is required"})
	}

	resp, err := h.svc.Refresh(input.RefreshToken)
	if err != nil {
		status := fiber.StatusUnauthorized
		if err.Error() == "internal server error" {
			status = fiber.StatusInternalServerError
		}
		return c.Status(status).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	return c.JSON(resp)
}
