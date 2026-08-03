package bookmark

import (
	"github.com/gofiber/fiber/v2"
)

func RegisterRoutes(rg fiber.Router, h *Handler, protectedMW fiber.Handler) {
	app := rg.Group("/bookmarks", protectedMW)

	app.Post("", h.Create)
	app.Get("/:title", h.GetOne)
	app.Get("/:title/:book_name", h.GetSome)
	app.Get("", h.GetList)
	app.Put("/:title/:book_name", h.UpdateAll)
	app.Delete("/:title/:book_name", h.DeleteParent)
}
