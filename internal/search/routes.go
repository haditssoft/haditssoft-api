package search

import "github.com/gofiber/fiber/v2"

func RegisterRoutes(rg fiber.Router) {
	app := rg.Group("/searchHadits")

	app.Post("/all/:column", SearchHadithAll)
	app.Post("/:kitabName/:column", GetSearchHadith)
}
