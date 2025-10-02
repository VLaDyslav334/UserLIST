package todo

type TodoList struct {
	ID          int    `json:"-"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

type UserList struct {
	ID          int    `json:"-"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Done        bool   `json:"done"`
}

type ListItem struct {
	ID     int
	ListID int
	ItemID int
}
