package todo

type User struct {
	ID       int    `json:"-"`
	Name     string `json:"name"`
	UserName string `json:"username"`
	Password string `json:"password"`
}
