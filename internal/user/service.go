package user

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"time"

	"github.com/haditssoft/haditssoft-backend/internal/shared/auth"
	"github.com/haditssoft/haditssoft-backend/internal/shared/response"
	"github.com/haditssoft/haditssoft-backend/models"

	"github.com/golang-jwt/jwt/v4"
	"github.com/jinzhu/copier"
	"gorm.io/gorm"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) GetOne(id string) (*response.UserResponseField, error) {
	model, err := s.repo.GetByID(id)
	if err != nil {
		return nil, err
	}
	resp := new(response.UserResponseField)
	copier.Copy(resp, model)
	return resp, nil
}

func (s *Service) Create(email, password, passwordConfirmation, path, ip string) (string, error) {
	user, code := s.repo.NewUserWithCode(email, password)
	active := false
	admin := false
	user.Active = &active
	user.Admin = &admin

	var createdUser models.User
	err := s.repo.Transaction(func(tx *gorm.DB) error {
		ctx := context.WithValue(context.Background(), "skip_before_create", true)
		if err := tx.WithContext(ctx).Create(user).Error; err != nil {
			return err
		}
		createdUser = *user

		activity := models.Activity{
			ActionURL: path,
			ReqMethod: "POST",
			Note:      "Create new user",
			IP:        ip,
			UserID:    user.ID,
		}
		if err := tx.Create(&activity).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return "", err
	}

	s.repo.SendVerificationCode([]string{createdUser.Email}, createdUser.Email, code)

	token := jwt.New(jwt.SigningMethodHS256)
	claims := token.Claims.(jwt.MapClaims)
	claims["email"] = createdUser.Email
	claims["user_id"] = createdUser.ID
	claims["exp"] = time.Now().Add(time.Hour * 72).Unix()

	t, err := token.SignedString([]byte(auth.JWTSecret()))
	if err != nil {
		return "", errors.New("failed to generate token")
	}

	return t, nil
}

func (s *Service) Verify(userID uint, code, path, ip string) error {
	model, err := s.repo.GetByID(userID)
	if err != nil {
		return err
	}

	if model.EmailVerifiedAt.Valid {
		return errors.New("Email already verified")
	}

	if model.VerificationCode != code {
		return errors.New("Invalid verification code")
	}

	if !model.VerificationCodeExpiresAt.Valid || time.Now().After(model.VerificationCodeExpiresAt.Time) {
		return errors.New("Verification code expired")
	}

	return s.repo.Transaction(func(tx *gorm.DB) error {
		model.EmailVerifiedAt = sql.NullTime{Time: time.Now(), Valid: true}
		active := true
		model.Active = &active
		model.VerificationCode = ""
		model.VerificationCodeExpiresAt = sql.NullTime{}

		if err := tx.Save(model).Error; err != nil {
			return err
		}

		activity := models.Activity{
			ActionURL: path,
			ReqMethod: "POST",
			Note:      "Verify email",
			IP:        ip,
			UserID:    model.ID,
		}
		return tx.Create(&activity).Error
	})
}

func (s *Service) Resend(userID uint) error {
	model, err := s.repo.GetByID(userID)
	if err != nil {
		return err
	}

	if model.EmailVerifiedAt.Valid {
		return errors.New("Email already verified")
	}

	if model.VerificationCodeExpiresAt.Valid {
		remaining := time.Until(model.VerificationCodeExpiresAt.Time)
		if remaining > 13*time.Minute {
			return errors.New("Please wait before requesting a new code")
		}
	}

	code := s.repo.GenerateVerificationCode()
	model.VerificationCode = code
	model.VerificationCodeExpiresAt = sql.NullTime{Time: time.Now().Add(15 * time.Minute), Valid: true}

	if err := s.repo.Save(model); err != nil {
		return err
	}

	s.repo.SendVerificationCode([]string{model.Email}, model.Email, code)
	return nil
}

func (s *Service) Update(targetID, callerID uint, email, active, admin, newPassword, path, ip string) (*response.UserResponseField, error) {
	var oldRecord models.User

	err := s.repo.Transaction(func(tx *gorm.DB) error {
		if err := s.repo.FirstWithLockTx(tx, &oldRecord, targetID); err != nil {
			return err
		}

		oldRecord.Email = email

		if newPassword != "" {
			oldRecord.Password = newPassword
		}

		if err := tx.Save(&oldRecord).Error; err != nil {
			return err
		}

		activity := models.Activity{
			ActionURL: path,
			ReqMethod: "PUT",
			Note:      "Update user",
			IP:        ip,
			UserID:    callerID,
		}
		return tx.Create(&activity).Error
	})
	if err != nil {
		return nil, err
	}

	resp := new(response.UserResponseField)
	copier.Copy(resp, &oldRecord)
	return resp, nil
}

func (s *Service) DeleteOne(id, callerID uint, path, ip string) error {
	var model models.User
	result := s.repo.DB().Where("id = ?", id).First(&model)
	if result.Error != nil {
		return result.Error
	}

	return s.repo.Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(&model).Error; err != nil {
			return err
		}

		activity := models.Activity{
			ActionURL: path,
			ReqMethod: "DELETE",
			Note:      "Delete a user",
			IP:        ip,
			UserID:    callerID,
		}
		return tx.Create(&activity).Error
	})
}

func (s *Service) ForgotPassword(email, path, ip string) error {
	model, err := s.repo.GetByEmail(email)
	if err != nil {
		return nil
	}

	if model.VerificationCodeExpiresAt.Valid {
		remaining := time.Until(model.VerificationCodeExpiresAt.Time)
		if remaining > 13*time.Minute {
			return errors.New("Please wait before requesting a new code")
		}
	}

	code := s.repo.GenerateVerificationCode()

	ctx := context.WithValue(context.Background(), "user_id", float64(model.ID))
	err = s.repo.TransactionWithContext(ctx, func(tx *gorm.DB) error {
		model.VerificationCode = code
		model.VerificationCodeExpiresAt = sql.NullTime{Time: time.Now().Add(15 * time.Minute), Valid: true}

		if err := tx.Save(model).Error; err != nil {
			return err
		}

		activity := models.Activity{
			ActionURL: path,
			ReqMethod: "POST",
			Note:      "Request password reset",
			IP:        ip,
			UserID:    model.ID,
		}
		return tx.Create(&activity).Error
	})

	if err != nil {
		return err
	}

	s.repo.SendVerificationCode([]string{model.Email}, model.Email, code)
	return nil
}

func (s *Service) ConfirmForgotPassword(email, code, newPassword, path, ip string) error {
	model, err := s.repo.GetByEmail(email)
	if err != nil {
		return err
	}

	if model.VerificationCode != code {
		return errors.New("Invalid verification code")
	}

	if !model.VerificationCodeExpiresAt.Valid || time.Now().After(model.VerificationCodeExpiresAt.Time) {
		return errors.New("Verification code expired")
	}

	ctx := context.WithValue(context.Background(), "user_id", float64(model.ID))
	return s.repo.TransactionWithContext(ctx, func(tx *gorm.DB) error {
		model.Password = newPassword
		model.VerificationCode = ""
		model.VerificationCodeExpiresAt = sql.NullTime{}

		if err := tx.Save(model).Error; err != nil {
			return err
		}

		activity := models.Activity{
			ActionURL: path,
			ReqMethod: "POST",
			Note:      "Reset password via code",
			IP:        ip,
			UserID:    model.ID,
		}
		return tx.Create(&activity).Error
	})
}

func (s *Service) GetList(page, limit int, search string) ([]models.User, int64, error) {
	return s.repo.SearchUser(page, limit, search)
}

func (s *Service) GetSome(rawIDs string) ([]responseField, error) {
	ids, err := s.repo.ParseJSONMapIDs(rawIDs)
	if err != nil {
		return nil, err
	}
	return s.repo.GetSomeUsers(ids)
}

func (s *Service) AdminCreate(email, password, activeStr, adminStr string, callerID uint, path, ip string) (*response.UserResponseField, error) {
	user := new(models.User)
	user.Email = email
	user.Password = password
	active := activeStr == "true"
	admin := adminStr == "true"
	user.Active = &active
	user.Admin = &admin

	err := s.repo.Transaction(func(tx *gorm.DB) error {
		ctx := context.WithValue(context.Background(), "user_id", float64(callerID))
		if err := tx.WithContext(ctx).Create(user).Error; err != nil {
			return err
		}

		activity := models.Activity{
			ActionURL: path,
			ReqMethod: "POST",
			Note:      "Create new user",
			IP:        ip,
			UserID:    callerID,
		}
		if err := tx.Create(&activity).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	s.repo.SendConfirmation([]string{user.Email}, "Email Confirmation")

	resp := new(response.UserResponseField)
	copier.Copy(resp, user)
	return resp, nil
}

func (s *Service) AdminUpdate(id uint, email, active, admin, newPassword string, callerID uint, path, ip string) (*response.UserResponseField, error) {
	var oldRecord models.User

	err := s.repo.Transaction(func(tx *gorm.DB) error {
		if err := s.repo.FirstWithLockTx(tx, &oldRecord, id); err != nil {
			return err
		}

		if email != "" {
			oldRecord.Email = email
		}
		if active != "" {
			isActive := active == "true"
			oldRecord.Active = &isActive
		}
		if admin != "" {
			isAdmin := admin == "true"
			oldRecord.Admin = &isAdmin
		}
		if newPassword != "" {
			oldRecord.Password = newPassword
		}

		if err := tx.Save(&oldRecord).Error; err != nil {
			return err
		}

		activity := models.Activity{
			ActionURL: path,
			ReqMethod: "PUT",
			Note:      "Update user",
			IP:        ip,
			UserID:    callerID,
		}
		return tx.Create(&activity).Error
	})
	if err != nil {
		return nil, err
	}

	resp := new(response.UserResponseField)
	copier.Copy(resp, &oldRecord)
	return resp, nil
}

func (s *Service) AdminDeleteOne(id, callerID uint, path, ip string) error {
	var model models.User
	result := s.repo.DB().Where("id = ?", id).First(&model)
	if result.Error != nil {
		return result.Error
	}

	return s.repo.Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(&model).Error; err != nil {
			return err
		}

		activity := models.Activity{
			ActionURL: path,
			ReqMethod: "DELETE",
			Note:      "Delete a user",
			IP:        ip,
			UserID:    callerID,
		}
		return tx.Create(&activity).Error
	})
}

func (s *Service) AdminDeleteSome(rawIDs string, callerID uint, path, ip string) (string, error) {
	ids, err := s.repo.ParseJSONArray(rawIDs)
	if err != nil {
		return "", err
	}

	users, err := s.repo.FindUsersByIDs(ids)
	if err != nil {
		return "", err
	}
	if len(users) < 1 {
		return "", errors.New("not found")
	}

	var deletedIdsStr string
	err = s.repo.Transaction(func(tx *gorm.DB) error {
		for _, mod := range users {
			recordId := mod.ID
			if err := tx.Delete(&mod).Error; err != nil {
				return err
			}
			if deletedIdsStr != "" {
				deletedIdsStr += ","
			}
			deletedIdsStr += strconv.FormatUint(uint64(recordId), 10)
		}

		activity := models.Activity{
			ActionURL: path,
			ReqMethod: "DELETE",
			Note:      "Delete some users (" + deletedIdsStr + ")",
			IP:        ip,
			UserID:    callerID,
		}
		return tx.Create(&activity).Error
	})

	return deletedIdsStr, err
}
