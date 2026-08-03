package auth

import (
	"github.com/gofiber/fiber/v2"
)

func RegisterRoutes(rg fiber.Router, h *Handler, mws ...fiber.Handler) {
	app := rg.Group("/auths")

	app.Post("/login", h.Login)

	logoutGroup := app.Group("/logout")
	for _, mw := range mws {
		logoutGroup.Use(mw)
	}
	logoutGroup.Post("", h.Logout)

	identityGroup := app.Group("/identity")
	for _, mw := range mws {
		identityGroup.Use(mw)
	}
	identityGroup.Get("", h.Identity)

	app.Post("/refresh", h.Refresh)
}

func RegisterAdminRoutes(rg fiber.Router, h *AdminHandler, mws ...fiber.Handler) {
	app := rg.Group("/auths")

	app.Post("/login", h.Login)
	app.Post("/refresh", h.Refresh)

	logoutGroup := app.Group("/logout")
	for _, mw := range mws {
		logoutGroup.Use(mw)
	}
	logoutGroup.Post("", h.Logout)

	identityGroup := app.Group("/identity")
	for _, mw := range mws {
		identityGroup.Use(mw)
	}
	identityGroup.Get("", h.Identity)
}
