package middleware

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/golang-jwt/jwt/v5"
)

func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get the Authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"message": "No authorization header found",
			})
			c.Abort()
			return
		}

		// Check Bearer token format
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {

			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"message": "Invalid token format",
			})
			c.Abort()
			return
		}

		tokenString := parts[1]

		// Parse and validate the token
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			// Validate signing method
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return []byte(os.Getenv("JWT_SECRET")), nil
		})

		if err != nil || !token.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"message": "Invalid token",
			})
			c.Abort()
			return
		}

		// Extract claims and set in context if needed
		if claims, ok := token.Claims.(jwt.MapClaims); ok {
			c.Set("user_id", claims["user_id"])
			c.Set("username", claims["username"])
		}

		c.Next()
	}
}

// RateLimitMiddleware creates a new rate limiter middleware
func RateLimitMiddleware(rdb *redis.Client) gin.HandlerFunc {
	// Get rate limit configurations from environment
	limit, _ := strconv.Atoi(os.Getenv("RATE_LIMIT"))
	if limit == 0 {
		limit = 100 // default value
	}

	windowStr := os.Getenv("RATE_LIMIT_WINDOW")
	window, err := time.ParseDuration(windowStr)
	if err != nil {
		window = time.Minute // default value
	}

	return func(c *gin.Context) {
		// Get identifier for rate limiting
		// Use user_id if authenticated, otherwise use IP
		var identifier string
		if userId, exists := c.Get("user_id"); exists {
			identifier = fmt.Sprintf("ratelimit:user:%v", userId)
		} else {
			identifier = fmt.Sprintf("ratelimit:ip:%s", c.ClientIP())
		}

		ctx := context.Background()

		// Get current count
		count, err := rdb.Get(ctx, identifier).Int()
		if err != nil && err != redis.Nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Rate limiting error",
			})
			c.Abort()
			return
		}

		// If key doesn't exist, create it with count 0
		if err == redis.Nil {
			count = 0
		}

		if count >= limit {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"success": false,
				"message": "Rate limit exceeded",
			})
			c.Abort()
			return
		}

		// Increment counter and set expiry using pipeline
		pipe := rdb.Pipeline()
		pipe.Incr(ctx, identifier)
		if count == 0 {
			pipe.Expire(ctx, identifier, window)
		}
		_, err = pipe.Exec(ctx)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Rate limiting error",
			})
			c.Abort()
			return
		}

		// Add rate limit headers
		c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", limit))
		c.Header("X-RateLimit-Remaining", fmt.Sprintf("%d", limit-count-1))
		c.Header("X-RateLimit-Reset", fmt.Sprintf("%d", time.Now().Add(window).Unix()))

		c.Next()
	}
}
