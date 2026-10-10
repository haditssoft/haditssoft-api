package opencode

import (
	"github.com/haditssoft/haditssoft-backend/internal/shared/middleware"

	"github.com/gofiber/fiber/v2"
)

func RegisterRoutes(rg fiber.Router) {
	app := rg.Group("/ai")

	app.Post("/cron/translate/:kitabName", TranslateHadiths)
	app.Post("/cron/translate/bulk/:kitabName", TranslateHadithsBulk)
	app.Post("/cron/translate/bulk/id/:kitabName", TranslateHadithsBulkID)
	app.Post("/cron/translate/title/bulk/:kitabName", TranslateTitlesBulk)
	app.Post("/cron/translate/title/bulk/id/:kitabName", TranslateTitlesBulkID)
	app.Post("/cron/summarize/:kitabName", SummarizeHadiths)
	app.Post("/ask", middleware.Protected(), AskOpenCode)
}
