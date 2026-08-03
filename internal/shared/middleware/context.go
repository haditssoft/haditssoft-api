package middleware

import (
	"context"
	"github.com/haditssoft/haditssoft-backend/internal/shared/database"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v4"
)

func SetConexContext(c *fiber.Ctx) {
	token := c.Locals("user").(*jwt.Token)
	claims := token.Claims.(jwt.MapClaims)
	ctx := context.Background()
	ctx = context.WithValue(ctx, "fiber_ctx", c)
	ctx = context.WithValue(ctx, "user_id", claims["user_id"].(float64))
	ctx = context.WithValue(ctx, "email", claims["email"].(string))
	database.SetContext(ctx)
}
