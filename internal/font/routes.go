package font

import (
	"github.com/haditssoft/haditssoft-backend/internal/shared/middleware"

	"github.com/gofiber/fiber/v2"
)

func RegisterRoutes(rg fiber.Router, handler *Handler) {
	app := rg.Group("/fonts")

	app.Get("", middleware.Protected(), handler.GetOne)
	app.Put("", middleware.Protected(), handler.Update)
}
