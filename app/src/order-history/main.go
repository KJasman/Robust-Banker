package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"main/database"
	"main/middleware"
	"main/models"
	"main/service"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	dbHandler, err := database.NewTimescaleDBHandler()
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer dbHandler.Close()

	// Run migrations
	if err := dbHandler.RunMigrations(); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	// Initialize services
	txService := service.NewTransactionService(dbHandler)

	// Setup router
	r := gin.Default()

	// API Routes
	api := r.Group("/api/v1")
	api.Use(middleware.TokenAuthMiddleware())

	// Transaction routes
	transactions := api.Group("/transaction")
	transactions.GET("/getStockTransactions", func(c *gin.Context) {
		userID := c.GetString("userID")

		stockTxs, err := txService.GetStockTransactions(c.Request.Context(), userID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"data":    nil,
				"message": fmt.Sprintf("Failed to get stock transactions: %v", err),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    stockTxs,
		})
	})

	transactions.GET("/getWalletTransactions", func(c *gin.Context) {
		userID := c.GetString("userID")

		walletTxs, err := txService.GetWalletTransactions(c.Request.Context(), userID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"data":    nil,
				"message": fmt.Sprintf("Failed to get wallet transactions: %v", err),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    walletTxs,
		})
	})

	// Internal API for other services - not authenticated
	internal := r.Group("/internal")
	internal.POST("/recordStockTransaction", func(c *gin.Context) {
		var tx models.StockTransaction
		if err := c.ShouldBindJSON(&tx); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"data":    nil,
				"message": fmt.Sprintf("Invalid request: %v", err),
			})
			return
		}

		if err := txService.RecordStockTransaction(c.Request.Context(), &tx); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"data":    nil,
				"message": fmt.Sprintf("Failed to record stock transaction: %v", err),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    nil,
		})
	})

	internal.POST("/recordWalletTransaction", func(c *gin.Context) {
		var tx models.WalletTransaction
		if err := c.ShouldBindJSON(&tx); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"data":    nil,
				"message": fmt.Sprintf("Invalid request: %v", err),
			})
			return
		}

		if err := txService.RecordWalletTransaction(c.Request.Context(), &tx); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"data":    nil,
				"message": fmt.Sprintf("Failed to record wallet transaction: %v", err),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    nil,
		})
	})

	// Health check
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "UP",
			"time":   time.Now().Format(time.RFC3339),
		})
	})

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8082"
	}

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: r,
	}

	log.Printf("Starting order-history service on port %s", port)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Failed to start server: %v", err)
	}
}
