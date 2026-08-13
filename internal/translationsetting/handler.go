package translationsetting

import "github.com/gofiber/fiber/v2"

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) GetOne(c *fiber.Ctx) error {
	return h.svc.GetOne(c)
}

func (h *Handler) Update(c *fiber.Ctx) error {
	return h.svc.Update(c)
}
