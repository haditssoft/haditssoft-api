package user

import (
	"github.com/gofiber/fiber/v2"
)

func RegisterRoutes(rg fiber.Router, h *Handler, tokenOnlyMW fiber.Handler, protectedMW fiber.Handler) {
	app := rg.Group("/users")

	app.Post("/verify", tokenOnlyMW, h.Verify)
	app.Post("/verify/resend", tokenOnlyMW, h.Resend)
	app.Post("", h.Create)
	app.Post("/forgot-password", h.ForgotPassword)
	app.Post("/forgot-password/confirm", h.ConfirmForgotPassword)

	protected := app.Group("", protectedMW)
	protected.Get("/:id", h.GetOne)
	protected.Delete("/:id", h.DeleteOne)
	protected.Put("/:id", h.Update)
}

func RegisterAdminRoutes(rg fiber.Router, h *AdminHandler, mws ...fiber.Handler) {
	app := rg.Group("/users")

	for _, mw := range mws {
		app.Use(mw)
	}

	app.Get("/some", h.GetSome)
	app.Get("/:id", h.GetOne)
	app.Get("", h.GetList)
	app.Post("", h.Create)
	app.Delete("/:id", h.DeleteOne)
	app.Delete("", h.DeleteSome)
	app.Put("/:id", h.Update)
}
