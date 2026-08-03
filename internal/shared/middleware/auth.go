package middleware

import (
	"github.com/haditssoft/haditssoft-backend/internal/shared/auth"
	"github.com/haditssoft/haditssoft-backend/internal/shared/database"
	"github.com/haditssoft/haditssoft-backend/internal/shared/database/entities"
	"net/mail"

	"github.com/gofiber/fiber/v2"
	jwtware "github.com/gofiber/jwt/v3"
	"github.com/golang-jwt/jwt/v4"
)

func Protected() fiber.Handler {
	return jwtware.New(jwtware.Config{
		SigningKey:     []byte(auth.JWTSecret()),
		SuccessHandler: jwtSuccess,
		ErrorHandler:   jwtError,
	})
}

func TokenOnly() fiber.Handler {
	return jwtware.New(jwtware.Config{
		SigningKey: []byte(auth.JWTSecret()),
		SuccessHandler: func(c *fiber.Ctx) error {
			SetConexContext(c)
			return c.Next()
		},
		ErrorHandler: jwtError,
	})
}

func jwtSuccess(c *fiber.Ctx) error {
	token := c.Locals("user").(*jwt.Token)

	SetConexContext(c)

	var model entities.BlacklistToken
	result := database.DB.Select("id").Where("token = ?", token.Raw).Find(&model).Limit(1)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 0 {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"status": "error", "message": "Expired token", "data": nil})
	}

	uid := auth.UserID(c)

	var usrAc entities.User
	resultAc := database.DB.Select("id").Unscoped().Where("id = ?", uid).Where("active", false).Find(&usrAc).Limit(1)
	if resultAc.Error != nil {
		return resultAc.Error
	}
	if resultAc.RowsAffected != 0 {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"status": "error", "message": "Need email confirmation", "data": nil})
	}

	var usrEx entities.User
	resultEx := database.DB.Select("id").Unscoped().Where("id = ?", uid).Find(&usrEx).Limit(1)
	if resultEx.Error != nil {
		return resultEx.Error
	}

	var usr entities.User
	result = database.DB.Select("id").Unscoped().Where("id = ?", uid).Where("deletedAt IS NOT NULL").Find(&usr).Limit(1)
	if result.Error != nil {
		return result.Error
	}
	if resultEx.RowsAffected == 0 || result.RowsAffected == 1 {
		blModel := entities.BlacklistToken{
			Token: token.Raw,
		}
		if err := database.DB.Create(&blModel).Error; err != nil {
			return err
		}
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"status": "error", "message": "Deleted account", "data": nil})
	}
	return c.Next()
}

func jwtError(c *fiber.Ctx, err error) error {
	if err.Error() == "Missing or malformed JWT" {
		return c.Status(fiber.StatusBadRequest).
			JSON(fiber.Map{"status": "error", "message": "Missing or malformed JWT", "data": nil})
	}
	return c.Status(fiber.StatusUnauthorized).
		JSON(fiber.Map{"status": "error", "message": "Invalid or expired JWT", "data": nil})
}

func IsActive(c *fiber.Ctx) error {
	type LoginInput struct {
		Email string `json:"email"`
	}
	input := new(LoginInput)
	if err := c.BodyParser(input); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": err.Error(),
		})
	}

	_, err := mail.ParseAddress(input.Email)
	if err == nil {
		var usr entities.User
		errs := database.DB.Select("email").Where("email = ?", input.Email).First(&usr).Error
		if errs != nil {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"message": "record not found",
			})
		}
	} else {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"message": "record not found",
		})
	}

	return c.Next()
}
