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

func newReverseProxy(targetBase, stripPrefix string) gin.HandlerFunc {
	targetURL, err := url.Parse(targetBase)
	if err != nil {
		log.Fatalf("Invalid target base: %v", err)
	}
	proxy := httputil.NewSingleHostReverseProxy(targetURL)

	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		if strings.HasPrefix(req.URL.Path, stripPrefix) {
			// Remove the prefix from the path before forwarding
			req.URL.Path = strings.TrimPrefix(req.URL.Path, stripPrefix)
		}
	}

	return func(c *gin.Context) {
		// Forward user_id/user_type
		if userID, ok := c.Get("user_id"); ok {
			c.Request.Header.Set("X-User-ID", toString(userID))
		}
		if userType, ok := c.Get("user_type"); ok {
			c.Request.Header.Set("X-User-Type", toString(userType))
		}
		proxy.ServeHTTP(c.Writer, c.Request)
	}
}

// Helper
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

	// Global middlewares
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

	//----------------------------------------------------------------
	//  Authentication
	//   e.g. /authentication/register/customer
	//        /authentication/register/company
	//        /authentication/login
	//----------------------------------------------------------------
	authGroup := r.Group("/authentication")
	{
		authProxy := newReverseProxy(services["auth"].URL, "/authentication")
		authGroup.POST("/register", authProxy)
		authGroup.POST("/login", authProxy)
	}
	//----------------------------------------------------------------
	// Setup endpoints
	//   e.g. POST /setup/createStock        => wallet-portfolio service
	//        POST /setup/addStockToUser     => order-service
	//----------------------------------------------------------------
	setupGroup := r.Group("/setup")
	setupGroup.Use(middleware.AuthMiddleware())
	{
		// 1) createStock goes to wallet-portfolio
		setupGroup.POST("/createStock", newReverseProxy(services["wallet"].URL, "/setup"))

		// 2) addStockToUser goes to order-service
		setupGroup.POST("/addStockToUser", newReverseProxy(services["order"].URL, "/setup"))
	}

	//----------------------------------------------------------------
	// Engine endpoints
	//   e.g. POST /engine/placeStockOrder
	//        POST /engine/cancelStockTransaction
	//----------------------------------------------------------------
	engineGroup := r.Group("/engine")
	engineGroup.Use(middleware.AuthMiddleware())
	{
		engineProxy := newReverseProxy(services["order"].URL, "/engine")
		engineGroup.POST("/placeStockOrder", engineProxy)
		engineGroup.POST("/cancelStockTransaction", engineProxy)
	}

	//----------------------------------------------------------------
	// Transaction/Wallet endpoints
	//   e.g. /transaction/addMoneyToWallet
	//        /transaction/getWalletBalance
	//        /transaction/getStockPortfolio
	//----------------------------------------------------------------
	transactionGroup := r.Group("/transaction")
	transactionGroup.Use(middleware.AuthMiddleware())
	{
		walletProxy := newReverseProxy(services["wallet"].URL, "/transaction")
		transactionGroup.POST("/addMoneyToWallet", walletProxy)
		transactionGroup.GET("/getWalletBalance", walletProxy)
		transactionGroup.GET("/getStockPortfolio", walletProxy)

	}

	//----------------------------------------------------------------
	// Fallback for unknown routes
	//----------------------------------------------------------------
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
