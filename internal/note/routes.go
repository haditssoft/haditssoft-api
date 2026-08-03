package note

import (
	"github.com/haditssoft/haditssoft-backend/internal/shared/middleware"

	"github.com/gofiber/fiber/v2"
)

func RegisterRoutes(rg fiber.Router, handler *Handler) {
	app := rg.Group("/notes")

	app.Post("/:book_name/:hadith_id", middleware.Protected(), handler.Create)
	app.Get("/:book_name/:hadith_id", middleware.Protected(), handler.GetOne)
	app.Get("/validate-delete/:book_name/:hadith_id", middleware.Protected(), handler.ValidateDelete)
	app.Get("/:book_name", middleware.Protected(), handler.GetList)
	app.Put("/:book_name/:hadith_id", middleware.Protected(), handler.Update)
	app.Delete("/:book_name/:hadith_id", middleware.Protected(), handler.DeleteOne)
}
