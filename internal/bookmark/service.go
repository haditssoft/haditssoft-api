package bookmark

import (
	"errors"
	"strconv"
	"time"

	"gorm.io/gorm"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) GetList(userID uint) ([]string, error) {
	titles, err := s.repo.GetTitlesByUserID(userID)
	if err != nil {
		return nil, err
	}
	if len(titles) == 0 {
		return nil, errors.New("Bookmark not found")
	}
	return titles, nil
}

func (s *Service) GetOne(userID uint, title string) (map[string][]int, error) {
	if title == "" {
		return nil, errors.New("title parameter is required")
	}

	items, err := s.repo.GetItemsByTitle(userID, title)
	if err != nil {
		return nil, err
	}

	response := make(map[string][]int)
	for _, item := range items {
		response[item.BookName] = append(response[item.BookName], item.BookNumber)
	}
	return response, nil
}

func (s *Service) GetSome(userID uint, title, bookName string) ([]int64, error) {
	if title == "" {
		return nil, errors.New("title parameter is required")
	}
	if bookName == "" {
		return nil, errors.New("book name parameter is required")
	}

	return s.repo.GetBookNumbersByTitleAndBookName(userID, title, bookName)
}

func (s *Service) Create(userID uint, title, bookName string, bookNumber uint, path, ip string) error {
	now := time.Now()

	return s.repo.Transaction(func(tx *gorm.DB) error {
		bookmarkID := s.repo.FindBookmarkIDByUserAndTitle(userID, title)

		if bookmarkID == 0 {
			var err error
			bookmarkID, err = s.repo.CreateBookmarkTx(tx, userID, title, now)
			if err != nil {
				return err
			}
		}

		if err := s.repo.CreateBookmarkItemTx(tx, bookmarkID, bookName, bookNumber, now); err != nil {
			return err
		}

		if err := s.repo.CreateActivityTx(tx, userID, path, "POST", "Create new bookmark", ip); err != nil {
			return err
		}

		return nil
	})
}

func (s *Service) UpdateAll(userID uint, title, bookName string, payload []uint, path, ip string) error {
	if len(payload) == 0 {
		return errors.New("payload is required")
	}

	return s.repo.Transaction(func(tx *gorm.DB) error {
		bookmarkItemID, err := s.repo.FindBookmarkItemToDeleteTx(tx, userID, title, bookName, payload)
		if err != nil {
			return err
		}

		if bookmarkItemID == 0 {
			return errors.New("not found")
		}

		if err := s.repo.SoftDeleteBookmarkItemTx(tx, bookmarkItemID); err != nil {
			return err
		}

		note := "Delete a bookmark item with ID: " + strconv.Itoa(int(bookmarkItemID))
		if err := s.repo.CreateActivityTx(tx, userID, path, "PUT", note, ip); err != nil {
			return err
		}

		return nil
	})
}

func (s *Service) DeleteParent(userID uint, title, bookName, path, ip string) (int64, error) {
	var rowsAffected int64

	err := s.repo.Transaction(func(tx *gorm.DB) error {
		model, err := s.repo.FindBookmarkItemForDeleteTx(tx, userID, title, bookName)
		if err != nil {
			return err
		}

		result := tx.Delete(model)
		if result.Error != nil {
			return result.Error
		}
		rowsAffected = result.RowsAffected

		note := "Delete a bookmark with ID: " + strconv.Itoa(int(model.ID))
		if err := s.repo.CreateActivityTx(tx, userID, path, "DELETE", note, ip); err != nil {
			return err
		}

		return nil
	})

	return rowsAffected, err
}
