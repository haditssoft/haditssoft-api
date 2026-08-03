package validations

type BookmarkUpdate struct {
	ID       uint   `form:"id" json:"id" validate:"required,is_exists_db_dynamic_table_for_note=BookName id"`
	HadithID uint   `form:"hadith_id" json:"hadith_id" validate:"required,is_exists_db_dynamic_table=BookName Nomer"`
	Note     string `form:"note" json:"note" validate:"required,min=1"`
	BookName string `form:"book_name" json:"book_name" validate:"required"`
}
