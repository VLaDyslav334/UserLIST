package handler

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"UserLIST/pkg/service"
	mocks "UserLIST/pkg/service/mocks"
	"UserLIST/todo"
)

func TestHandler_signUp(t *testing.T) {
	// Setup
	type mockBehavior func(s *mocks.MockAuthorization, user todo.User)

	tests := []struct {
		name               string
		inputBody          string
		inputUser          todo.User
		mockBehavior       mockBehavior
		expectedStatusCode int
		expectedResponse   string
	}{
		{
			name:      "Valid",
			inputBody: `{"name":"Test","username":"test","password":"qwerty"}`,
			inputUser: todo.User{
				Name:     "Test",
				Username: "test",
				Password: "qwerty",
			},
			mockBehavior: func(s *mocks.MockAuthorization, user todo.User) {
				s.EXPECT().CreateUser(user).Return(1, nil)
			},
			expectedStatusCode: 200,
			expectedResponse:   `{"id":1}`,
		},
		{
			name:      "Service Error",
			inputBody: `{"name":"Test","username":"test","password":"qwerty"}`,
			inputUser: todo.User{
				Name:     "Test",
				Username: "test",
				Password: "qwerty",
			},
			mockBehavior: func(s *mocks.MockAuthorization, user todo.User) {
				s.EXPECT().CreateUser(user).Return(0, errors.New("service error"))
			},
			expectedStatusCode: 500,
			expectedResponse:   `{"message":"service error"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Init Dependencies
			c := gomock.NewController(t)
			defer c.Finish()

			auth := mocks.NewMockAuthorization(c)
			tt.mockBehavior(auth, tt.inputUser)

			services := &service.Service{Authorization: auth}
			handler := NewHandler(services)

			// Test Server
			r := gin.New()
			r.POST("/auth/sign-up", handler.signUp)

			// Test Request
			w := httptest.NewRecorder()
			req := httptest.NewRequest("POST", "/auth/sign-up",
				bytes.NewBufferString(tt.inputBody))
			req.Header.Set("Content-Type", "application/json")

			// Perform Request
			r.ServeHTTP(w, req)

			// Assert
			assert.Equal(t, tt.expectedStatusCode, w.Code)
			assert.Equal(t, tt.expectedResponse, w.Body.String())
		})
	}
}

func TestHandler_signIn(t *testing.T) {
	type mockBehavior func(s *mocks.MockAuthorization, input todo.User)

	tests := []struct {
		name               string
		inputBody          string
		inputUser          todo.User
		mockBehavior       mockBehavior
		expectedStatusCode int
		expectedResponse   string
	}{
		{
			name:      "Valid",
			inputBody: `{"username":"test","password":"qwerty"}`,
			inputUser: todo.User{
				Username: "test",
				Password: "qwerty",
			},
			mockBehavior: func(s *mocks.MockAuthorization, user todo.User) {
				s.EXPECT().GenerateToken(user.Username, user.Password).Return("token", nil)
			},
			expectedStatusCode: 200,
			expectedResponse:   `{"token":"token"}`,
		},
		{
			name:      "Invalid Credentials",
			inputBody: `{"username":"test","password":"invalid"}`,
			inputUser: todo.User{
				Username: "test",
				Password: "invalid",
			},
			mockBehavior: func(s *mocks.MockAuthorization, user todo.User) {
				s.EXPECT().GenerateToken(user.Username, user.Password).Return("", errors.New("invalid credentials"))
			},
			expectedStatusCode: 500,
			expectedResponse:   `{"message":"invalid credentials"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := gomock.NewController(t)
			defer c.Finish()

			auth := mocks.NewMockAuthorization(c)
			tt.mockBehavior(auth, tt.inputUser)

			services := &service.Service{Authorization: auth}
			handler := NewHandler(services)

			r := gin.New()
			r.POST("/auth/sign-in", handler.signIn)

			w := httptest.NewRecorder()
			req := httptest.NewRequest("POST", "/auth/sign-in",
				bytes.NewBufferString(tt.inputBody))
			req.Header.Set("Content-Type", "application/json")

			r.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatusCode, w.Code)
			assert.Equal(t, tt.expectedResponse, w.Body.String())
		})
	}
}

func TestHandler_createList(t *testing.T) {
	type mockBehavior func(s *mocks.MockTodoList, userId int, list todo.TodoList)

	tests := []struct {
		name               string
		inputBody          string
		inputList          todo.TodoList
		userId             int
		mockBehavior       mockBehavior
		expectedStatusCode int
		expectedResponse   string
	}{
		{
			name:      "Valid",
			inputBody: `{"title":"test list","description":"test description"}`,
			inputList: todo.TodoList{
				Title:       "test list",
				Description: "test description",
			},
			userId: 1,
			mockBehavior: func(s *mocks.MockTodoList, userId int, list todo.TodoList) {
				s.EXPECT().Create(userId, list).Return(1, nil)
			},
			expectedStatusCode: http.StatusOK,
			expectedResponse:   `{"id":1}`,
		},
		{
			name:      "Empty Title",
			inputBody: `{"title":"","description":"test description"}`,
			inputList: todo.TodoList{
				Title:       "",
				Description: "test description",
			},
			userId: 1,
			mockBehavior: func(s *mocks.MockTodoList, userId int, list todo.TodoList) {
			},
			expectedStatusCode: http.StatusBadRequest,
			expectedResponse:   `{"message":"Key: 'TodoList.Title' Error:Field validation for 'Title' failed on the 'required' tag"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Init Dependencies
			c := gomock.NewController(t)
			defer c.Finish()

			todoList := mocks.NewMockTodoList(c)
			tt.mockBehavior(todoList, tt.userId, tt.inputList)

			services := &service.Service{TodoList: todoList}
			handler := NewHandler(services)

			// Test Server
			gin.SetMode(gin.TestMode) // Important to set test mode
			r := gin.New()
			r.POST("/api/lists", func(c *gin.Context) {
				c.Set(userCtx, tt.userId)
				handler.createList(c)
			})

			// Test Request
			w := httptest.NewRecorder()
			req := httptest.NewRequest("POST", "/api/lists",
				bytes.NewBufferString(tt.inputBody))
			req.Header.Set("Content-Type", "application/json")

			// Perform Request
			r.ServeHTTP(w, req)

			// Assert
			assert.Equal(t, tt.expectedStatusCode, w.Code)
			assert.Equal(t, tt.expectedResponse, w.Body.String())
		})
	}
}

func TestHandler_getAllLists(t *testing.T) {
	type mockBehavior func(s *mocks.MockTodoList, userId int)

	tests := []struct {
		name               string
		userId             int
		mockBehavior       mockBehavior
		expectedStatusCode int
		expectedResponse   string
	}{
		{
			name:   "Valid",
			userId: 1,
			mockBehavior: func(s *mocks.MockTodoList, userId int) {
				lists := []todo.TodoList{
					{Id: 1, Title: "list1", Description: "description1"},
					{Id: 2, Title: "list2", Description: "description2"},
				}
				s.EXPECT().GetAll(userId).Return(lists, nil)
			},
			expectedStatusCode: 200,
			expectedResponse:   `{"data":[{"id":1,"title":"list1","description":"description1"},{"id":2,"title":"list2","description":"description2"}]}`,
		},
		{
			name:   "Service Error",
			userId: 1,
			mockBehavior: func(s *mocks.MockTodoList, userId int) {
				s.EXPECT().GetAll(userId).Return(nil, errors.New("service error"))
			},
			expectedStatusCode: 500,
			expectedResponse:   `{"message":"service error"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := gomock.NewController(t)
			defer c.Finish()

			todoList := mocks.NewMockTodoList(c)
			tt.mockBehavior(todoList, tt.userId)

			services := &service.Service{TodoList: todoList}
			handler := NewHandler(services)

			r := gin.New()
			r.GET("/api/lists", func(c *gin.Context) {
				c.Set(userCtx, tt.userId)
				handler.getAllLists(c)
			})

			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/api/lists", nil)

			r.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatusCode, w.Code)
			assert.Equal(t, tt.expectedResponse, w.Body.String())
		})
	}
}
