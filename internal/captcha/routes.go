package captcha

import "github.com/gofiber/fiber/v2"

func RegisterRoutes(rg fiber.Router) {
	app := rg.Group("/verifyreCaptcha")

	app.Post("/", VerifyreCaptcha, SendTelegramReport)
}
