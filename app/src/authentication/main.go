package main // main backend server 

import (
	// MODULES
	"database/sql" // SQL database
	"fmt" 		   // I/O
	"log"		   // logs errors and messages
	"net/http"	   // http requests
	"os"		   // read environment variables
	"time"		   // time-related operations 

	// LIBRARIES
	"github.com/gin-gonic/gin"      // Gin framework for handling HTTP requests
	"github.com/golang-jwt/jwt/v5"	// JWT authentication
	"github.com/joho/godotenv"		// environment variables
	_ "github.com/lib/pq"			// PostgreSQL database driver 
	"golang.org/x/crypto/bcrypt"	// Hash and Verify passwords securely 
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
	// Database Connection: define how the app connects to databases
	// retrieve db connection details directly from the operating system's environment variables
	host := os.Getenv("DB_HOST") // hostname / IP address of db server 
	port := os.Getenv("DB_PORT") // port number database listens on (PostgreSQL: 5432, MySQL: 3306)
	user := os.Getenv("DB_USER") // databse username for authentification, different microservice may have different
	password := os.Getenv("DB_PASSWORD") // user authentification to db server, can only access to its own table
	dbname := os.Getenv("DB_NAME") // databse name to connect to
	sslmode := os.Getenv("DB_SSLMODE") // define whether SSL encryption is enabled

	// return string is used by sql.Open("postgres", connStr) to establish a database connection
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		host, port, user, password, dbname, sslmode,
	)
}

func initDB() error {
	// Build connection string from environment variables
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

// One Migration 
func applyMigrations() error {
	// Automate database migration by reading and executing SQL file that define db structure 
	// Ensure the app starts and db exist and structure is set up correctly
	// Return: nil if correct schema, otherwise, triggers error

	// Read migration file: instead of hardcoding SQL commands here, keep them in separate file 
	//                      allows us to modify the db schema without changing this file
	// Separate sql file is only used for Schema setup, not include queries 
	// that are a part of application's dynamic logic, hard code in that case 
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

// AUTHENTIFICATION
func generateToken(userID int, username string) (string, error) {
	// Generate token for authenticated user (successfully log in)
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id":  userID,
		"username": username,
		"exp":      time.Now().Add(time.Hour * 12).Unix(), // 12 hour expiration
		"iat":      time.Now().Unix(), // issued time
	})

	// Sign "token" using JWT_SECRET key from environment variables
	tokenString, err := token.SignedString([]byte(os.Getenv("JWT_SECRET")))
	if err != nil {
		return "", err
	}

	return tokenString, nil // return signed JWT token, send to frontend 
	// frontend stores the token in local storage or HTTP headers
	// Every future API request includes:
	// Authorization: Bearer <JWT_TOKEN>
}

func registerHandler(c *gin.Context) {
	// Take one parameter: request context from Gin 

	// Parse "Register" Request  
	var req RegisterRequest // declare variable "req" struct type "RegisterRequest"
	// read request body and maps it to "req"
	// c helps us access request data (JSON input) and send responses (JSON output).
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{Success: false, Message: "Invalid request body"})
		// if parsing failed, return 400 Bad Request
		return
	}

	// Hash password
	// securely encrypt pw by bcrypt hashing before storing in db 
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil { // if hashing fails, send HTTP 500 Internal Server Error 
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
	// Successful Registration 
	c.JSON(http.StatusOK, Response{Success: true, Data: nil})
}

func loginHandler(c *gin.Context) {
	var req LoginRequest // same as req := RegisterRequest{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{Success: false, Message: "Invalid request body"})
		return
	}

	// Get user from database
	var user User // parse existing related data to "username" into "user"
	err := db.QueryRow(
		"SELECT id, username, password FROM users WHERE username = $1",
		req.Username, // username from request
	).Scan(&user.ID, &user.Username, &user.Password) // extract retrieved data to "user" struct

	if err != nil { // 401 Unauthorized 
		c.JSON(http.StatusUnauthorized, Response{Success: false, Message: "Invalid credentials"})
		return
	}

	// Check password: compare pw from request and pw stored in db 
	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password))
	if err != nil {
		c.JSON(http.StatusUnauthorized, Response{Success: false, Message: "Invalid credentials"})
		return
	}

	// Generate JWT token to Successful Login 
	token, err := generateToken(user.ID, user.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Message: "Error generating token"})
		return
	}

	c.JSON(http.StatusOK, Response{Success: true, Data: Token{SignedToken: token}})
}

func main() {
	r := gin.Default()

	// Authentication endpoints
	r.POST("/authentication/register", registerHandler)
	r.POST("/authentication/login", loginHandler)

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
