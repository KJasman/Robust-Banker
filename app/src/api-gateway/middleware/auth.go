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
		authHeader := c.GetHeader("Authorization")
		tokenHeader := c.GetHeader("token") // Jmeter case
		var tokenString string

		if authHeader == "" && tokenHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"message": "No authorization header found",
			})
			c.Abort()
			return
		}

		if authHeader != "" {
			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || parts[0] != "Bearer" {
				c.JSON(http.StatusUnauthorized, gin.H{
					"success": false,
					"message": "Invalid Postman token format",
				})
				c.Abort()
				return
			}

			tokenString = parts[1]
			fmt.Println("Token received:", tokenString)

		} else if tokenHeader != "" { // Jmeter case
			tokenString = tokenHeader
		}

		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
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

		if claims, ok := token.Claims.(jwt.MapClaims); ok {
			c.Set("user_id", claims["user_id"])
			c.Set("username", claims["username"])
			c.Set("user_type", claims["user_type"])
			fmt.Println("User Type:", claims["user_type"])
		}

		c.Next()
	}
}

func RateLimitMiddleware(rdb *redis.Client) gin.HandlerFunc {
	limit, _ := strconv.Atoi(os.Getenv("RATE_LIMIT"))
	if limit == 0 {
		limit = 100
	}

	windowStr := os.Getenv("RATE_LIMIT_WINDOW")
	window, err := time.ParseDuration(windowStr)
	if err != nil {
		window = time.Minute
	}

	return func(c *gin.Context) {
		var identifier string
		if userId, exists := c.Get("user_id"); exists {
			identifier = fmt.Sprintf("ratelimit:user:%v", userId)
		} else {
			identifier = fmt.Sprintf("ratelimit:ip:%s", c.ClientIP())
		}

		ctx := context.Background()

		count, err := rdb.Get(ctx, identifier).Int()
		if err != nil && err != redis.Nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Rate limiting error",
			})
			c.Abort()
			return
		}

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

		c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", limit))
		c.Header("X-RateLimit-Remaining", fmt.Sprintf("%d", limit-count-1))
		c.Header("X-RateLimit-Reset", fmt.Sprintf("%d", time.Now().Add(window).Unix()))

		c.Next()
	}
}
