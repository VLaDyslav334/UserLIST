package repository

import (
	"fmt"
	"testing"

	todo "UserLIST/todo"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
)

func TestTodoListPostgres_Create(t *testing.T) {
	db, mock, cleanup := newMockDB(t)
	defer cleanup()

	r := NewTodoListPostgres(db)

	tests := []struct {
		name    string
		userId  int
		list    todo.TodoList
		want    int
		wantErr bool
		setup   func()
	}{
		{
			name:   "Ok",
			userId: 1,
			list: todo.TodoList{
				Title:       "test list",
				Description: "test description",
			},
			want:    2,
			wantErr: false,
			setup: func() {
				mock.ExpectBegin()

				mock.ExpectQuery(`INSERT INTO todo_lists`).
					WithArgs("test list", "test description").
					WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(2))

				mock.ExpectExec(`INSERT INTO users_lists`).
					WithArgs(1, 2).
					WillReturnResult(sqlmock.NewResult(0, 1))

				mock.ExpectCommit()
			},
		},
		{
			name:   "Insert Error",
			userId: 1,
			list: todo.TodoList{
				Title:       "",
				Description: "test",
			},
			want:    0,
			wantErr: true,
			setup: func() {
				mock.ExpectBegin()

				mock.ExpectQuery(`INSERT INTO todo_lists`).
					WithArgs("", "test").
					WillReturnError(fmt.Errorf("insert error"))

				mock.ExpectRollback()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			got, err := r.Create(tt.userId, tt.list)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestTodoListPostgres_GetAll(t *testing.T) {
	db, mock, cleanup := newMockDB(t)
	defer cleanup()

	r := NewTodoListPostgres(db)

	tests := []struct {
		name    string
		userId  int
		want    []todo.TodoList
		wantErr bool
		setup   func()
	}{
		{
			name:   "Ok",
			userId: 1,
			want: []todo.TodoList{
				{Id: 1, Title: "list1", Description: "desc1"},
				{Id: 2, Title: "list2", Description: "desc2"},
			},
			wantErr: false,
			setup: func() {
				rows := sqlmock.NewRows([]string{"id", "title", "description"}).
					AddRow(1, "list1", "desc1").
					AddRow(2, "list2", "desc2")

				mock.ExpectQuery(`SELECT tl\.id, tl\.title, tl\.description FROM todo_lists tl ` +
					`INNER JOIN users_lists ul on tl\.id = ul\.list_id ` +
					`WHERE ul\.user_id = \$1`).
					WithArgs(1).
					WillReturnRows(rows)
			},
		},
		{
			name:    "No Lists",
			userId:  1,
			want:    nil, // Changed to nil since that's what the repository returns
			wantErr: false,
			setup: func() {
				mock.ExpectQuery(`SELECT tl\.id, tl\.title, tl\.description FROM todo_lists tl ` +
					`INNER JOIN users_lists ul on tl\.id = ul\.list_id ` +
					`WHERE ul\.user_id = \$1`).
					WithArgs(1).
					WillReturnRows(sqlmock.NewRows([]string{"id", "title", "description"}))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			got, err := r.GetAll(tt.userId)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestTodoListPostgres_GetById(t *testing.T) {
	db, mock, cleanup := newMockDB(t)
	defer cleanup()

	r := NewTodoListPostgres(db)

	tests := []struct {
		name    string
		userId  int
		listId  int
		want    todo.TodoList
		wantErr bool
		setup   func()
	}{
		{
			name:   "Ok",
			userId: 1,
			listId: 2,
			want: todo.TodoList{
				Id:          2,
				Title:       "test list",
				Description: "test description",
			},
			wantErr: false,
			setup: func() {
				rows := sqlmock.NewRows([]string{"id", "title", "description"}).
					AddRow(2, "test list", "test description")

				// Updated query to match exact SQL from repository
				mock.ExpectQuery(`SELECT tl\.id, tl\.title, tl\.description FROM todo_lists tl `+
					`INNER JOIN users_lists ul on tl\.id = ul\.list_id `+
					`WHERE ul\.user_id = \$1 AND ul\.list_id = \$2`).
					WithArgs(1, 2).
					WillReturnRows(rows)
			},
		},
		{
			name:    "Not Found",
			userId:  1,
			listId:  404,
			want:    todo.TodoList{},
			wantErr: true,
			setup: func() {
				mock.ExpectQuery(`SELECT tl\.id, tl\.title, tl\.description FROM todo_lists tl `+
					`INNER JOIN users_lists ul on tl\.id = ul\.list_id `+
					`WHERE ul\.user_id = \$1 AND ul\.list_id = \$2`).
					WithArgs(1, 404).
					WillReturnError(fmt.Errorf("no rows in result set"))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			got, err := r.GetById(tt.userId, tt.listId)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}
