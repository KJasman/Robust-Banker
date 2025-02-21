package main

import (
	"fmt"
	"log"
	"main/middleware"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"

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
	"wallet": {URL: "http://wallet-service:8083"},
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
		// Forward user_id/user_type as X-User-ID / X-User-Type
		if userID, ok := c.Get("user_id"); ok {
			c.Request.Header.Set("X-User-ID", toString(userID))
		}
		if userType, ok := c.Get("user_type"); ok {
			c.Request.Header.Set("X-User-Type", toString(userType))
		}

		proxy.ServeHTTP(c.Writer, c.Request)
	}
}

func toString(val interface{}) string {
	if s, ok := val.(string); ok {
		return s
	}
	return strings.TrimSpace(strings.ReplaceAll(fmt.Sprintf("%v", val), "<nil>", ""))
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

	// Authentication endpoints (public)
	//
	// e.g.:
	// POST /authentication/register/customer
	// POST /authentication/register/company
	// POST /authentication/login
	authGroup := r.Group("/authentication")
	{
		authProxy := newReverseProxy(services["auth"].URL, "/authentication")
		authGroup.POST("/register/customer", authProxy)
		authGroup.POST("/register/company", authProxy)
		authGroup.POST("/login", authProxy)
	}

	// Orders endpoints (protected). E.g. GET/POST /orders/*path
	orders := r.Group("/orders")
	orders.Use(middleware.AuthMiddleware())
	{
		orderProxy := newReverseProxy(services["order"].URL, "/orders")
		orders.GET("/*path", orderProxy)
		orders.POST("/*path", orderProxy)
		orders.PUT("/*path", orderProxy)
		orders.DELETE("/*path", orderProxy)
	}

	// Wallet / Transaction endpoints (protected)
	// E.g. /transaction/addMoneyToWallet, etc.
	transaction := r.Group("/transaction")
	transaction.Use(middleware.AuthMiddleware())
	{
		walletProxy := newReverseProxy(services["wallet"].URL, "/transaction")
		transaction.POST("/addMoneyToWallet", walletProxy)
		transaction.GET("/getWalletBalance", walletProxy)
		transaction.GET("/getWalletTransactions", walletProxy)
		transaction.GET("/getStockPortfolio", walletProxy)
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
