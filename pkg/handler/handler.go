package handler

import (
	"github.com/gin-gonic/gin"
)

type Handler struct {
}

func (h *Handler) InitRoutes() *gin.Engine {
	router := gin.New()

	auth := router.Group("/auth")
	{
		auth.POST("/sing-up", h.singUp)
		auth.POST("/sing-in", h.singIn)
	}
	api := router.Group("/api")
	{
		list := api.Group("/list")
		{
			list.GET("/:id", h.getListById)
			list.GET("/", h.getAllLists)
			list.POST("/", h.createList)
			list.PUT("/:id", h.updateList)
			list.DELETE("/:id", h.deleteList)

			items := router.Group("/items")
			{
				items.GET("/:id", h.getItemById)
				items.GET("/", h.getAllItems)
				items.POST("/", h.createItem)
				items.PUT("/:id", h.updateItem)
				items.DELETE("/:id", h.deleteItem)
			}

		}
	}
	return router
}
