package validations

type LastReadUpdate struct {
	Number   uint   `form:"number" json:"number" validate:"required,min=1,is_exists_db_dynamic_table=BookName Nomer"`
	BookName string `form:"book_name" json:"book_name" validate:"required"`
}
