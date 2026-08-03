package hadithadmin

import (
	"github.com/haditssoft/haditssoft-backend/internal/shared/middleware"

	"github.com/gofiber/fiber/v2"
)

func RegisterRoutes(rg fiber.Router) {
	app := rg.Group("", middleware.Protected(), middleware.IsAdmin)

	app.Get("/:kitabName", GetList)
	app.Get("/:kitabName/:number", GetOne)
	app.Post("/:kitabName", PostOne)
	app.Put("/:kitabName/:number", PutOne)
	app.Delete("/:kitabName/:number", DeleteOne)
}
