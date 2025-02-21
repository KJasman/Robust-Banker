package main // main backend server

import (
	// MODULES
	"database/sql" // SQL database
	"fmt"          // I/O
	"log"          // logs errors and messages
	"net/http"     // http requests
	"os"           // read environment variables
	"time"         // time-related operations

	// LIBRARIES
	"github.com/gin-gonic/gin"     // Gin framework for handling HTTP requests
	"github.com/golang-jwt/jwt/v5" // JWT authentication
	"github.com/joho/godotenv"     // environment variables
	_ "github.com/lib/pq"          // PostgreSQL database driver
	"golang.org/x/crypto/bcrypt"   // Hash and Verify passwords securely
)

// DATABASE: define expected request bodies for LOGIN and REGISTRATION
type User struct {
	//  name	 dtype  json field mapping: ensure json request/response uses "..."
	ID       int    `json:"id"`
	Username string `json:"user_name"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

type LoginRequest struct {
	Username string `json:"user_name"`
	Password string `json:"password"`
}

type RegisterRequest struct {
	Username string `json:"user_name"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

type Token struct {
	SignedToken string `json:"token"`
}

type Response struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data"`
	Message string      `json:"message,omitempty"`
}

// SET UP
var db *sql.DB

func buildDatabaseURL() string {
	host := os.Getenv("DB_HOST")
	port := os.Getenv("DB_PORT")
	user := os.Getenv("DB_USER")
	password := os.Getenv("DB_PASSWORD")
	dbname := os.Getenv("DB_NAME")
	sslmode := os.Getenv("DB_SSLMODE")

	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		host, port, user, password, dbname, sslmode,
	)
}

func initDB() error {
	connStr := buildDatabaseURL()

	var err error
	db, err = sql.Open("postgres", connStr) // create database connection
	if err != nil {
		return fmt.Errorf("error opening database: %v", err)
	}

	// Configure connection pool to limit database resource usage
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Test connection
	if err = db.Ping(); err != nil {
		return fmt.Errorf("error connecting to the database: %v", err)
	}

	return applyMigrations() // if no error occurs, call next function
}

func applyMigrations() error {
	migration, err := os.ReadFile("migrations/001_create_users_table.sql")
	if err != nil {
		return fmt.Errorf("error reading migration file: %v", err)
	}

	// Execute migration
	_, err = db.Exec(string(migration))
	if err != nil {
		return fmt.Errorf("error applying migration: %v", err)
	}

	log.Println("Database migrations applied successfully")
	return nil
}

func init() {
	// Load .env file and set the environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file not found")
	}

	// Initialize database connection, read environment variables
	if err := initDB(); err != nil {
		log.Fatal("Failed to initialize database:", err)
	}
}

func generateToken(userID int, username string) (string, error) {
	// Generate token for authenticated user (successfully log in)
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id":  userID,
		"username": username,
		"exp":      time.Now().Add(time.Hour * 12).Unix(), // 12 hour expiration
		"iat":      time.Now().Unix(),                     // issued time
	})

	// Sign "token" using JWT_SECRET key from environment variables
	tokenString, err := token.SignedString([]byte(os.Getenv("JWT_SECRET")))
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

func registerHandler(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{Success: false, Data: nil, Message: "Invalid request body"})
		return
	}
	var exists bool
	err := db.QueryRow("SELECT EXISTS (SELECT 1 FROM users WHERE username = $1)", req.Username).Scan(&exists)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Data: nil})
		return
	}

	if exists {
		c.JSON(http.StatusBadRequest, Response{Success: false, Data: nil})
		return
	}
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Message: "Error processing password"})
		return
	}

	// Insert new user
	var userID int
	err = db.QueryRow(
		"INSERT INTO users (username, password, name) VALUES ($1, $2, $3) RETURNING id",
		req.Username,
		string(hashedPassword),
		req.Name,
	).Scan(&userID)

	// Failed Registration
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Message: "Error creating user"})
		return
	}
	c.JSON(http.StatusOK, Response{Success: true, Data: nil})
}

func loginHandler(c *gin.Context) {
	var req LoginRequest // same as req := RegisterRequest{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{Success: false, Data: nil})
		return
	}

	var user User
	err := db.QueryRow(
		"SELECT id, username, password FROM users WHERE username = $1",
		req.Username,
	).Scan(&user.ID, &user.Username, &user.Password) // extract retrieved data to "user" struct

	if err != nil {
		c.JSON(http.StatusBadRequest, Response{Success: false, Data: nil})
		return
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password))
	if err != nil {
		c.JSON(http.StatusBadRequest, Response{Success: false, Data: nil})
		return
	}

	token, err := generateToken(user.ID, user.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Data: nil, Message: "Error generating token"})
		return
	}

	c.JSON(http.StatusOK, Response{Success: true, Data: Token{SignedToken: token}})
}

func main() {
	r := gin.Default()

	// Authentication endpoints
	r.POST("/register", registerHandler)
	r.POST("/login", loginHandler)

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Server starting on port %s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatal(err)
	}
}
