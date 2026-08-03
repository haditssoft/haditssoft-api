package middleware

import (
	"github.com/haditssoft/haditssoft-backend/internal/shared/auth"
	"github.com/haditssoft/haditssoft-backend/internal/shared/database"
	"github.com/haditssoft/haditssoft-backend/internal/shared/database/entities"

	"github.com/gofiber/fiber/v2"
)

func IsAdmin(c *fiber.Ctx) error {
	uid := auth.UserID(c)

	var user entities.User
	result := database.DB.Select("admin").Where("id = ?", uid).First(&user)
	if result.Error != nil || user.Admin == nil || !*user.Admin {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"message": "not an admin",
		})
	}
	return c.Next()
}
