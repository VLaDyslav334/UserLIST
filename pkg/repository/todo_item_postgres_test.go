package repository

import (
	"fmt"
	"testing"

	todo "UserLIST/todo"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
)

func newMockDB(t *testing.T) (*sqlx.DB, sqlmock.Sqlmock, func()) {
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock database: %v", err)
	}
	db := sqlx.NewDb(sqlDB, "sqlmock")
	cleanup := func() {
		db.Close()
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("there were unfulfilled expectations: %v", err)
		}
	}
	return db, mock, cleanup
}

func TestTodoItemPostgres_Create(t *testing.T) {
	db, mock, cleanup := newMockDB(t)
	defer cleanup()

	r := NewTodoItemPostgres(db)

	tests := []struct {
		name    string
		listId  int
		item    todo.TodoItem
		want    int
		wantErr bool
		setup   func()
	}{
		{
			name:   "Ok",
			listId: 1,
			item: todo.TodoItem{
				Title:       "test title",
				Description: "test description",
			},
			want:    2,
			wantErr: false,
			setup: func() {
				mock.ExpectBegin()

				mock.ExpectQuery(`INSERT INTO\s*todo_items\s*\(title, description\)\s*`+
					`values\s*\(\$1, \$2\)\s*RETURNING\s*id`).
					WithArgs("test title", "test description").
					WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(2))

				mock.ExpectExec(`INSERT INTO\s*lists_items\s*\(list_id, item_id\)\s*`+
					`values\s*\(\$1, \$2\)`).
					WithArgs(1, 2).
					WillReturnResult(sqlmock.NewResult(0, 1))

				mock.ExpectCommit()
			},
		},
		{
			name:   "Transaction Error",
			listId: 1,
			item: todo.TodoItem{
				Title:       "test title",
				Description: "test description",
			},
			want:    0,
			wantErr: true,
			setup: func() {
				mock.ExpectBegin()

				mock.ExpectQuery(`INSERT INTO\s*todo_items\s*\(title, description\)\s*`+
					`values\s*\(\$1, \$2\)\s*RETURNING\s*id`).
					WithArgs("test title", "test description").
					WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(2))

				mock.ExpectExec(`INSERT INTO\s*lists_items\s*\(list_id, item_id\)\s*`+
					`values\s*\(\$1, \$2\)`).
					WithArgs(1, 2).
					WillReturnError(fmt.Errorf("foreign key violation"))

				mock.ExpectRollback()
			},
		},
		{
			name:   "Empty Title",
			listId: 1,
			item: todo.TodoItem{
				Title:       "",
				Description: "test description",
			},
			want:    0,
			wantErr: true,
			setup: func() {
				mock.ExpectBegin()

				mock.ExpectQuery(`INSERT INTO\s*todo_items\s*\(title, description\)\s*`+
					`values\s*\(\$1, \$2\)\s*RETURNING\s*id`).
					WithArgs("", "test description").
					WillReturnError(fmt.Errorf("title cannot be empty"))

				mock.ExpectRollback()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			got, err := r.Create(tt.listId, tt.item)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestTodoItemPostgres_GetAll(t *testing.T) {
	db, mock, cleanup := newMockDB(t)
	defer cleanup()

	r := NewTodoItemPostgres(db)

	tests := []struct {
		name    string
		userId  int
		listId  int
		want    []todo.TodoItem
		wantErr bool
		setup   func()
	}{
		{
			name:   "Ok",
			userId: 1,
			listId: 1,
			want: []todo.TodoItem{
				{Id: 1, Title: "title1", Description: "description1", Done: true},
				{Id: 2, Title: "title2", Description: "description2", Done: false},
			},
			wantErr: false,
			setup: func() {
				rows := sqlmock.NewRows([]string{"id", "title", "description", "done"}).
					AddRow(1, "title1", "description1", true).
					AddRow(2, "title2", "description2", false)
				// match any SELECT from todo_items (case-insensitive)
				mock.ExpectQuery("(?i)select.*from.*todo_items").WillReturnRows(rows)
			},
		},
		{
			name:    "Query error",
			userId:  1,
			listId:  1,
			want:    nil,
			wantErr: true,
			setup: func() {
				mock.ExpectQuery("(?i)select.*from.*todo_items").WillReturnError(fmt.Errorf("query failed"))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			got, err := r.GetAll(tt.userId, tt.listId)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestTodoItemPostgres_GetById(t *testing.T) {
	db, mock, cleanup := newMockDB(t)
	defer cleanup()

	r := NewTodoItemPostgres(db)

	tests := []struct {
		name    string
		userId  int
		itemId  int
		want    todo.TodoItem
		wantErr bool
		setup   func()
	}{
		{
			name:   "Ok",
			userId: 1,
			itemId: 2,
			want: todo.TodoItem{
				Id:          2,
				Title:       "test title",
				Description: "test description",
				Done:        true,
			},
			wantErr: false,
			setup: func() {
				rows := sqlmock.NewRows([]string{"id", "title", "description", "done"}).
					AddRow(2, "test title", "test description", true)

				// Updated query to match the actual query from repository
				mock.ExpectQuery(`SELECT ti\.id, ti\.title, ti\.description, ti\.done FROM\s*todo_items ti\s*`+
					`INNER JOIN lists_items li on li\.item_id = ti\.id\s*`+
					`INNER JOIN users_lists ul on ul\.list_id = li\.list_id\s*`+
					`WHERE ti\.id = \$1 AND ul\.user_id = \$2`).
					WithArgs(2, 1).
					WillReturnRows(rows)
			},
		},
		{
			name:    "Not Found",
			userId:  1,
			itemId:  404,
			want:    todo.TodoItem{},
			wantErr: true,
			setup: func() {
				// Updated query pattern for not found case
				mock.ExpectQuery(`SELECT ti\.id, ti\.title, ti\.description, ti\.done FROM\s*todo_items ti\s*`+
					`INNER JOIN lists_items li on li\.item_id = ti\.id\s*`+
					`INNER JOIN users_lists ul on ul\.list_id = li\.list_id\s*`+
					`WHERE ti\.id = \$1 AND ul\.user_id = \$2`).
					WithArgs(404, 1).
					WillReturnError(fmt.Errorf("no rows in result set"))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			got, err := r.GetById(tt.userId, tt.itemId)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestTodoItemPostgres_Delete(t *testing.T) {
	db, mock, cleanup := newMockDB(t)
	defer cleanup()

	r := NewTodoItemPostgres(db)

	tests := []struct {
		name    string
		userId  int
		itemId  int
		wantErr bool
		setup   func()
	}{
		{
			name:    "Ok",
			userId:  1,
			itemId:  2,
			wantErr: false,
			setup: func() {
				// Updated query pattern to match actual query with USING clause
				mock.ExpectExec(`DELETE FROM\s*todo_items ti\s*`+
					`USING lists_items li, users_lists ul\s*`+
					`WHERE ti\.id = li\.item_id\s*`+
					`AND li\.list_id = ul\.list_id\s*`+
					`AND ul\.user_id = \$1\s*`+
					`AND ti\.id = \$2`).
					WithArgs(1, 2).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
		},
		{
			name:    "Not Found",
			userId:  1,
			itemId:  404,
			wantErr: true,
			setup: func() {
				// Updated query pattern for not found case
				mock.ExpectExec(`DELETE FROM\s*todo_items ti\s*`+
					`USING lists_items li, users_lists ul\s*`+
					`WHERE ti\.id = li\.item_id\s*`+
					`AND li\.list_id = ul\.list_id\s*`+
					`AND ul\.user_id = \$1\s*`+
					`AND ti\.id = \$2`).
					WithArgs(1, 404).
					WillReturnError(fmt.Errorf("no rows affected"))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			err := r.Delete(tt.userId, tt.itemId)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestTodoItemPostgres_Update(t *testing.T) {
	db, mock, cleanup := newMockDB(t)
	defer cleanup()

	r := NewTodoItemPostgres(db)

	title := "updated title"
	description := "updated description"
	done := true

	tests := []struct {
		name    string
		userId  int
		itemId  int
		input   todo.UpdateItemInput
		wantErr bool
		setup   func()
	}{
		{
			name:   "Update All Fields",
			userId: 1,
			itemId: 2,
			input: todo.UpdateItemInput{
				Title:       &title,
				Description: &description,
				Done:        &done,
			},
			wantErr: false,
			setup: func() {
				mock.ExpectExec(`UPDATE\s*todo_items ti SET\s*`+
					`title=\$1, description=\$2, done=\$3\s*`+
					`FROM lists_items li, users_lists ul\s*`+
					`WHERE ti\.id = li\.item_id\s*`+
					`AND li\.list_id = ul\.list_id\s*`+
					`AND ul\.user_id = \$4\s*`+
					`AND ti\.id = \$5`).
					WithArgs(title, description, done, 1, 2).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
		},
		{
			name:   "Update Title Only",
			userId: 1,
			itemId: 2,
			input: todo.UpdateItemInput{
				Title: &title,
			},
			wantErr: false,
			setup: func() {
				mock.ExpectExec(`UPDATE\s*todo_items ti SET\s*`+
					`title=\$1\s*`+
					`FROM lists_items li, users_lists ul\s*`+
					`WHERE ti\.id = li\.item_id\s*`+
					`AND li\.list_id = ul\.list_id\s*`+
					`AND ul\.user_id = \$2\s*`+
					`AND ti\.id = \$3`).
					WithArgs(title, 1, 2).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
		},
		{
			name:   "Not Found",
			userId: 1,
			itemId: 404,
			input: todo.UpdateItemInput{
				Title: &title,
			},
			wantErr: true,
			setup: func() {
				mock.ExpectExec(`UPDATE\s*todo_items ti SET\s*`+
					`title=\$1\s*`+
					`FROM lists_items li, users_lists ul\s*`+
					`WHERE ti\.id = li\.item_id\s*`+
					`AND li\.list_id = ul\.list_id\s*`+
					`AND ul\.user_id = \$2\s*`+
					`AND ti\.id = \$3`).
					WithArgs(title, 1, 404).
					WillReturnError(fmt.Errorf("no rows affected"))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			err := r.Update(tt.userId, tt.itemId, tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
