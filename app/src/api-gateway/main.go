package main

import (
	"log"
	"main/middleware"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"

	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/joho/godotenv"
)

type ServiceConfig struct {
	URL string
}

var services = map[string]ServiceConfig{
	"auth":   {URL: "http://auth-service:8080"},
	"order":  {URL: "http://order-service:8081"},
	"wallet": {URL: "http://wallet-service:8083"}, // Ensure port matches your wallet container
}

func newReverseProxy(targetBase string, stripPrefix string) gin.HandlerFunc {
	targetURL, err := url.Parse(targetBase)
	if err != nil {
		log.Fatalf("Invalid target base: %v", err)
	}
	proxy := httputil.NewSingleHostReverseProxy(targetURL)

	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		if strings.HasPrefix(req.URL.Path, stripPrefix) {
			req.URL.Path = strings.TrimPrefix(req.URL.Path, stripPrefix)
		}
	}

	return func(c *gin.Context) {
		// If the AuthMiddleware stored "user_id" and "user_type" in gin.Context,
		// forward them as X-User-ID and X-User-Type
		if userID, ok := c.Get("user_id"); ok {
			c.Request.Header.Set("X-User-ID", toString(userID))
		}
		if userType, ok := c.Get("user_type"); ok {
			c.Request.Header.Set("X-User-Type", toString(userType))
		}

		proxy.ServeHTTP(c.Writer, c.Request)
	}
}

// toString is a small helper to convert interface{} to string.
func toString(val interface{}) string {
	if s, ok := val.(string); ok {
		return s
	}
	return strings.TrimSpace(strings.ReplaceAll((fmt.Sprintf("%v", val)), "<nil>", ""))
}

func main() {
	_ = godotenv.Load()

	rdb := redis.NewClient(&redis.Options{
		Addr: "redis:6379",
	})
	r := gin.Default()

	r.Use(middleware.RateLimitMiddleware(rdb))
	r.Use(func(c *gin.Context) {
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Next()
	})

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy"})
	})

	authGroup := r.Group("/api/v1/auth")
	{
		authProxy := newReverseProxy(services["auth"].URL, "/api/v1/auth")
		authGroup.POST("/register/customer", authProxy)
		authGroup.POST("/register/company", authProxy)
		authGroup.POST("/login", authProxy)
	}

	orderGroup := r.Group("/api/v1/orders")
	orderGroup.Use(middleware.AuthMiddleware())
	{
		orderProxy := newReverseProxy(services["order"].URL, "")
		orderGroup.GET("/*path", orderProxy)
		orderGroup.POST("/*path", orderProxy)
		orderGroup.PUT("/*path", orderProxy)
		orderGroup.DELETE("/*path", orderProxy)
	}

	transactionGroup := r.Group("/api/v1/transaction")
	transactionGroup.Use(middleware.AuthMiddleware())
	{
		walletProxy := newReverseProxy(services["wallet"].URL, "")
		transactionGroup.POST("/addMoneyToWallet", walletProxy)
		transactionGroup.GET("/getWalletBalance", walletProxy)
		transactionGroup.GET("/getWalletTransactions", walletProxy)
		transactionGroup.GET("/getStockPortfolio", walletProxy)
	}

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
