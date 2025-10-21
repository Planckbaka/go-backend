package main

import (
	"net/http"

	"github.com/Planckbaka/go-backend/internal/database"
	"github.com/gin-gonic/gin"
)

func main() {

	// Initial the database
	database.InitDatabase()

	// Create a router
	// gin.Default use 2 middlewares which are Logger() and Recovery()
	// If you don't like using them. you can create a router by gin.New()
	router := gin.Default()

	router.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "pong",
		})
	})
	err := router.Run(":8080")
	if err != nil {
		return
	} // listens on 0.0.0.0:8080 by default
}
