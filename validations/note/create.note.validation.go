package validations

type NoteCreate struct {
	HadithID uint   `form:"hadith_id" json:"hadith_id" validate:"required,is_exists_db_dynamic_table=BookName Nomer,is_not_exists_db_dynamic_table_for_note=BookName hadith_id"`
	Note     string `form:"note" json:"note" validate:"required,min=1"`
	BookName string `form:"book_name" json:"book_name" validate:"required"`
}
