package validations

type BookmarkCreate struct {
	UserID uint               `form:"user_id" json:"user_id" validate:"required"`
	Title  string             `form:"title" json:"title" validate:"required,min=3"`
	Items  BookmarkItemCreate `json:"items" validate:"required"`
}

type BookmarkItemCreate struct {
	BookName   string `form:"book_name" json:"book_name" validate:"required"`
	BookNumber uint   `form:"book_number" json:"book_number" validate:"required"`
}
