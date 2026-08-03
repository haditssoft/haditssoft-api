package validations

type NoteDelete struct {
	HadithID uint   `form:"hadith_id" json:"hadith_id" validate:"required,is_exists_db_dynamic_table=BookName Nomer,is_exists_db_dynamic_table_for_note=BookName hadith_id"`
	BookName string `form:"book_name" json:"book_name" validate:"required"`
}
