package note

import (
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) GetList(c *fiber.Ctx, bookName string, userID uint) error {
	if bookName == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "book_name parameter is required",
		})
	}

	notes, err := s.repo.GetList(bookName, userID)
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{"notes": notes})
}

func (s *Service) GetOne(c *fiber.Ctx, bookName string, hadithID uint, userID uint) error {
	if bookName == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "book_name parameter is required",
		})
	}

	container, err := s.repo.GetOne(bookName, hadithID, userID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"message": "Note not found",
			})
		}
		return err
	}

	return c.JSON(container)
}

func (s *Service) Create(c *fiber.Ctx, bookName string, hadithID uint, note string, userID uint) error {
	if bookName == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "book_name parameter is required",
		})
	}

	err := s.repo.Create(bookName, hadithID, note, userID, c.Path(), c.IP())
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"status": "error", "message": "Internal server error", "data": err.Error()})
	}

	return c.SendStatus(fiber.StatusNoContent)
}

func (s *Service) Update(c *fiber.Ctx, id uint, hadithID uint, note string, bookName string, userID uint, path, ip string) error {
	err := s.repo.Update(bookName, id, hadithID, note, userID, path, ip)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"status": "error", "message": "Internal server error", "data": err.Error()})
	}

	return c.SendStatus(fiber.StatusNoContent)
}

func (s *Service) FindNoteID(bookName string, hadithID uint, userID uint) (uint, error) {
	return s.repo.FindNoteIDByHadithAndUser(bookName, hadithID, userID)
}

func (s *Service) DeleteOne(c *fiber.Ctx, bookName string, hadithID uint, userID uint, path, ip string) error {
	if bookName == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "book_name parameter is required",
		})
	}

	err := s.repo.DeleteOne(bookName, hadithID, userID, path, ip)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"status": "error", "message": "Internal server error", "data": err.Error()})
	}

	return c.JSON(fiber.Map{"status": "success", "note": "Note deleted successfully"})
}
