package captcha

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/gofiber/fiber/v2"
)

type RecaptchaResp struct {
	Success     bool     `json:"success"`
	ChallengeTS string   `json:"challenge_ts,omitempty"`
	Hostname    string   `json:"hostname,omitempty"`
	ErrorCodes  []string `json:"error-codes,omitempty"`
	Score       float64  `json:"score,omitempty"`
	Action      string   `json:"action,omitempty"`
}

func VerifyreCaptcha(c *fiber.Ctx) error {
	type RequestBody struct {
		Token string `json:"token"`
	}

	var reqBody RequestBody

	if err := c.BodyParser(&reqBody); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	formBody := fmt.Sprintf("secret=%s&response=%s", os.Getenv("RECAPTCHA_KEY"), reqBody.Token)

	a := fiber.AcquireAgent()
	req := a.Request()
	req.Header.SetMethod(fiber.MethodPost)
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.SetBodyString(formBody)
	req.SetRequestURI("https://www.google.com/recaptcha/api/siteverify")

	defer fiber.ReleaseAgent(a)

	if err := a.Parse(); err != nil {
		log.Println("recaptcha request error:", err)
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
			"error": "verification failed",
		})
	}

	code, resBody, errs := a.Bytes()
	if len(errs) > 0 {
		return c.Status(code).JSON(fiber.Map{
			"error": errs,
		})
	}

	var gr RecaptchaResp
	if err := json.Unmarshal(resBody, &gr); err != nil {
		log.Println("recaptcha json decode error:", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "invalid verification response",
		})
	}

	if !gr.Success {
		return c.Status(code).JSON(fiber.Map{
			"success": false,
			"errors":  gr.ErrorCodes,
		})
	}

	return c.Next()
}

func SendTelegramReport(c *fiber.Ctx) error {
	type RequestBody struct {
		BookInfo   string `json:"bookInfo"`
		ReportText string `json:"reportText"`
	}

	var reqBody RequestBody

	if err := c.BodyParser(&reqBody); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	TELEGRAM_TOKEN := os.Getenv("TELEGRAM_BOT_TOKEN")
	CHAT_ID := os.Getenv("TELEGRAM_CHAT_ID")
	TELEGRAM_API := `https://api.telegram.org/bot` + TELEGRAM_TOKEN + `/sendMessage`

	a := fiber.AcquireAgent()
	a.JSON(fiber.Map{
		"chat_id":    CHAT_ID,
		"text":       "*" + reqBody.BookInfo + "*\n\n" + reqBody.ReportText,
		"parse_mode": "Markdown",
	})
	req := a.Request()
	req.Header.SetMethod(fiber.MethodPost)
	req.SetRequestURI(TELEGRAM_API)

	defer fiber.ReleaseAgent(a)

	if err := a.Parse(); err != nil {
		log.Println("recaptcha request error:", err)
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
			"error": "verification failed",
		})
	}

	code, resBody, errs := a.Bytes()
	if len(errs) > 0 {
		return c.Status(code).JSON(fiber.Map{
			"error": errs,
		})
	}

	if code != 200 {
		var rb map[string]interface{}
		if err := json.Unmarshal(resBody, &rb); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		return c.Status(code).JSON(fiber.Map{
			"error": rb["description"],
		})
	}

	return c.SendStatus(code)
}
