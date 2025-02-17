package main

import (
	"fmt"
	"log"
	"main/middleware"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/joho/godotenv"
)

type ServiceConfig struct {
	URL string
}

var (
	services = map[string]ServiceConfig{
		"auth":   {URL: "http://auth-service:8080"},
		"order":  {URL: "http://order-service:8081"},
		"wallet": {URL: "http://wallet-service:8082"},
	}
)

func createReverseProxy(targetURL string) gin.HandlerFunc {
	url, err := url.Parse(targetURL)
	if err != nil {
		log.Fatalf("Error parsing URL %s: %v", targetURL, err)
	}

	proxy := httputil.NewSingleHostReverseProxy(url)

	return func(c *gin.Context) {
		// Extract user_id from context (set by AuthMiddleware)
		if userID, exists := c.Get("user_id"); exists {
			c.Request.Header.Set("X-User-ID", fmt.Sprintf("%v", userID)) // Forward user_id as a header
		}
		if userType, exists := c.Get("user_type"); exists {
			c.Request.Header.Set("X-User-Type", fmt.Sprintf("%v", userType)) // Forward user_type as a header
		}

		proxy.ServeHTTP(c.Writer, c.Request)
	}
}

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found")
	}

	// Initialize Redis client
	rdb := redis.NewClient(&redis.Options{
		Addr: "redis:6379",
	})

	r := gin.Default()

	// Add global rate limiting
	r.Use(middleware.RateLimitMiddleware(rdb))

	// Add basic security headers
	r.Use(func(c *gin.Context) {
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Next()
	})

	// Health check endpoint
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "healthy",
		})
	})

	// Public authentication endpoints
	auth := r.Group("/api/v1/auth")
	{
		authProxy := createReverseProxy(services["auth"].URL)
		auth.POST("/register", authProxy)
		auth.POST("/login", authProxy)
	}

	// Protected order endpoints
	orders := r.Group("/api/v1/orders")
	orders.Use(middleware.AuthMiddleware())
	{
		orderProxy := createReverseProxy(services["order"].URL)
		orders.GET("/*path", orderProxy)
		orders.POST("/*path", orderProxy)
		orders.PUT("/*path", orderProxy)
		orders.DELETE("/*path", orderProxy)
	}

	// Protected wallet endpoints
	wallet := r.Group("/api/v1/wallet")
	wallet.Use(middleware.AuthMiddleware())
	{
		walletProxy := createReverseProxy(services["wallet"].URL)
		wallet.GET("/*path", walletProxy)
		wallet.POST("/*path", walletProxy)
		wallet.PUT("/*path", walletProxy)
	}

	// Error handler
	r.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "Route not found",
		})
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8000"
	}

	log.Printf("API Gateway starting on port %s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("Error starting server: %v", err)
	}
}
