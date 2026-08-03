package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v4"
	"golang.org/x/crypto/bcrypt"
)

const (
	AccessTokenExpiry  = 15 * time.Minute
	RefreshTokenExpiry = 7 * 24 * time.Hour
	RefreshTokenLength = 64
)

func JWTSecret() string {
	s := os.Getenv("JWT_SECRET")
	if s == "" {
		log.Fatal("JWT_SECRET environment variable is not set")
	}
	return s
}

func ValidToken(t *jwt.Token, id uint) bool {
	claims := t.Claims.(jwt.MapClaims)
	uid := uint(claims["user_id"].(float64))
	return uid == id
}

func CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

func UserID(c *fiber.Ctx) uint {
	token := c.Locals("user").(*jwt.Token)
	claims := token.Claims.(jwt.MapClaims)
	return uint(claims["user_id"].(float64))
}

func Email(c *fiber.Ctx) string {
	token := c.Locals("user").(*jwt.Token)
	claims := token.Claims.(jwt.MapClaims)
	return claims["email"].(string)
}

func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 14)
	return string(bytes), err
}

func GenerateAccessToken(userID uint, email string) (string, error) {
	token := jwt.New(jwt.SigningMethodHS256)
	claims := token.Claims.(jwt.MapClaims)
	claims["email"] = email
	claims["user_id"] = userID
	claims["exp"] = time.Now().Add(AccessTokenExpiry).Unix()
	return token.SignedString([]byte(JWTSecret()))
}

func GenerateRefreshToken() (plainToken, hashedToken string, err error) {
	b := make([]byte, RefreshTokenLength)
	if _, err := rand.Read(b); err != nil {
		return "", "", fmt.Errorf("failed to generate refresh token: %w", err)
	}
	plainToken = hex.EncodeToString(b)
	hashedToken = HashToken(plainToken)
	return plainToken, hashedToken, nil
}

func HashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}
