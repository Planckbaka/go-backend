package main

import (
	"fmt"
	"net/http"

	"github.com/Planckbaka/go-backend/internal/database"
	"github.com/Planckbaka/go-backend/internal/handler"
	"github.com/gin-gonic/gin"
)

func main() {

	// Initial the database
	database.InitDatabase()

	// Create a router
	// gin.Default use 2 middlewares which are Logger() and Recovery()
	// If you don't like using them. you can create a router by gin.New()
	router := gin.Default()

	// Load the HTML templates
	router.LoadHTMLGlob("templates/*")

	// Define a route for the root path "/"
	router.GET("/", func(c *gin.Context) {
		// Render the "index.html" template
		c.HTML(http.StatusOK, "index.html", gin.H{
			"title": "Hello World",
		})
	})

	router.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "pong",
		})
	})

	v1 := router.Group("/api/v1")
	{
		v1.POST("/files/upload", handler.UploadMultipleFiles)
	}

	fmt.Println("Starting server on http://localhost:8080")
	err := router.Run(":8080")
	if err != nil {
		return
	} // listens on 0.0.0.0:8080 by default
}
