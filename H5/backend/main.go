package main

import (
	"github.com/gin-gonic/gin"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"h5-backend/handlers" // Assuming handlers package for routes
	"h5-backend/models"
)

var db *gorm.DB

func main() {
	// Placeholder for remote DB connection - replace with actual remote MySQL details
	dsn := "root:rootpass@tcp(mysql:3306)/customer_service_db?charset=utf8mb4&parseTime=True&loc=Local"
	var err error
	db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		panic("failed to connect database")
	}

	// Auto-migrate models
	db.AutoMigrate(&models.MiniApp{}, &models.CustomerService{}, &models.Assignment{}, &models.User{}, &models.Message{}, &models.Config{})

	r := gin.Default()
	
	// CORS middleware
	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})
	
	r.Static("/uploads", "./uploads") // Serve images from /uploads

	// Setup admin routes
	handlers.SetupAdminRoutes(r, db)

	// Setup chat routes
	handlers.SetupChatRoutes(r, db)

	// TODO: Add WebSocket for chat, user message handling, etc.

	r.Run(":8080") // Listen on port 8080
}
