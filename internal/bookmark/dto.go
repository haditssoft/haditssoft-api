package bookmark

type BookmarkCreateInput struct {
	Title string `json:"title"`
	Items struct {
		BookName   string `json:"book_name"`
		BookNumber uint   `json:"book_number"`
	} `json:"items"`
}

type BookmarkUpdateAllInput struct {
	BookNumbers []uint `json:"book_numbers"`
}
