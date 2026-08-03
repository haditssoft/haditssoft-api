package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/haditssoft/haditssoft-backend/internal/shared/auth"
	"github.com/haditssoft/haditssoft-backend/internal/shared/database"
	"github.com/haditssoft/haditssoft-backend/internal/shared/database/entities"

	"gorm.io/gorm"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Login(email, password, ip, path string) (*LoginResponse, error) {
	user, err := s.repo.GetUserByEmail(email)
	if err != nil {
		return nil, errors.New("record not found")
	}
	if user == nil {
		return nil, errors.New("record not found")
	}
	if !auth.CheckPasswordHash(password, user.Password) {
		return nil, errors.New("record not found")
	}

	accessToken, err := auth.GenerateAccessToken(user.ID, user.Email)
	if err != nil {
		return nil, errors.New("failed to generate access token")
	}

	plainRT, hashedRT, err := auth.GenerateRefreshToken()
	if err != nil {
		return nil, errors.New("failed to generate refresh token")
	}

	refreshToken := entities.RefreshToken{
		UserID:    user.ID,
		TokenHash: hashedRT,
		ExpiresAt: time.Now().Add(auth.RefreshTokenExpiry),
	}
	if err := s.repo.CreateRefreshToken(&refreshToken); err != nil {
		return nil, errors.New("failed to create refresh token")
	}

	if err := s.repo.CreateActivity(user.ID, path, "POST", "Login", ip); err != nil {
		return nil, errors.New("failed to log activity")
	}

	return &LoginResponse{
		Status:       "success",
		Message:      "Success login",
		Token:        accessToken,
		RefreshToken: plainRT,
	}, nil
}

func (s *Service) AdminLogin(email, password, ip, path string) (*LoginResponse, error) {
	user, err := s.repo.GetActiveAdminByEmail(email)
	if err != nil {
		return nil, errors.New("record not found")
	}
	if user == nil {
		return nil, errors.New("record not found")
	}
	if !auth.CheckPasswordHash(password, user.Password) {
		return nil, errors.New("record not found")
	}

	accessToken, err := auth.GenerateAccessToken(user.ID, user.Email)
	if err != nil {
		return nil, errors.New("failed to generate access token")
	}

	plainRT, hashedRT, err := auth.GenerateRefreshToken()
	if err != nil {
		return nil, errors.New("failed to generate refresh token")
	}

	refreshToken := entities.RefreshToken{
		UserID:    user.ID,
		TokenHash: hashedRT,
		ExpiresAt: time.Now().Add(auth.RefreshTokenExpiry),
	}
	if err := s.repo.CreateRefreshToken(&refreshToken); err != nil {
		return nil, errors.New("failed to create refresh token")
	}

	if err := s.repo.CreateActivity(user.ID, path, "POST", "Login", ip); err != nil {
		return nil, errors.New("failed to log activity")
	}

	return &LoginResponse{
		Status:       "success",
		Message:      "Success login",
		Token:        accessToken,
		RefreshToken: plainRT,
	}, nil
}

func (s *Service) Logout(userID uint, tokenRaw, ip, path string) error {
	return database.DB.Transaction(func(tx *gorm.DB) error {
		if err := s.repo.CreateBlacklistToken(tokenRaw); err != nil {
			return err
		}
		return s.repo.CreateActivityTx(tx, userID, path, "POST", "Logout", ip)
	})
}

func (s *Service) Identity(userID uint) (*IdentityResponse, error) {
	user, err := s.repo.GetUserByID(userID)
	if err != nil {
		return nil, err
	}
	return &IdentityResponse{
		ID:    fmt.Sprintf("%d", user.ID),
		Email: user.Email,
	}, nil
}

func (s *Service) Refresh(refreshToken string) (*LoginResponse, error) {
	tokenHash := auth.HashToken(refreshToken)

	record, err := s.repo.FindRefreshTokenByHash(tokenHash)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("invalid refresh token")
		}
		return nil, errors.New("internal server error")
	}

	if record.IsUsed {
		s.repo.RevokeAllRefreshTokens(record.UserID)
		return nil, errors.New("refresh token reuse detected, all tokens revoked")
	}

	if time.Now().After(record.ExpiresAt) {
		s.repo.ExpireRefreshToken(record)
		return nil, errors.New("refresh token expired")
	}

	s.repo.ExpireRefreshToken(record)

	user, err := s.repo.GetUserFromRefreshToken(record)
	if err != nil {
		return nil, errors.New("user not found")
	}

	newAccessToken, err := auth.GenerateAccessToken(user.ID, user.Email)
	if err != nil {
		return nil, errors.New("failed to generate access token")
	}

	newPlainRT, newHashedRT, err := auth.GenerateRefreshToken()
	if err != nil {
		return nil, errors.New("failed to generate refresh token")
	}

	newRefreshToken := entities.RefreshToken{
		UserID:    user.ID,
		TokenHash: newHashedRT,
		ExpiresAt: time.Now().Add(auth.RefreshTokenExpiry),
	}
	if err := s.repo.CreateRefreshToken(&newRefreshToken); err != nil {
		return nil, errors.New("internal server error")
	}

	return &LoginResponse{
		Status:       "success",
		Message:      "Token refreshed",
		Token:        newAccessToken,
		RefreshToken: newPlainRT,
	}, nil
}
