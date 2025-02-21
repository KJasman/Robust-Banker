package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
)

const (
	UserTypeCustomer = "CUSTOMER"
	UserTypeCompany  = "COMPANY"
)

type BaseUser struct {
	ID       int    `json:"id"`
	Username string `json:"user_name"`
	Password string `json:"password"`
	UserType string `json:"user_type"`
}

type CustomerUser struct {
	BaseUser
	Name       string `json:"name"`
	CustomerID string `json:"customer_id"`
}

type CompanyUser struct {
	BaseUser
	CompanyName string `json:"company_name"`
	CompanyID   string `json:"company_id"`
}

type LoginRequest struct {
	Username string `json:"user_name"`
	Password string `json:"password"`
}

type RegisterCustomerRequest struct {
	Username string `json:"user_name"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

type RegisterCompanyRequest struct {
	Username    string `json:"user_name"`
	Password    string `json:"password"`
	CompanyName string `json:"company_name"`
}

type Token struct {
	SignedToken string `json:"token"`
}

type Response struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data"`
	Message string      `json:"message,omitempty"`
}

var db *sql.DB

func buildDatabaseURL() string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		os.Getenv("DB_HOST"),
		os.Getenv("DB_PORT"),
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_NAME"),
		os.Getenv("DB_SSLMODE"),
	)
}

func initDB() error {
	connStr := buildDatabaseURL()
	var err error
	db, err = sql.Open("postgres", connStr)
	if err != nil {
		return fmt.Errorf("error opening database: %v", err)
	}
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)
	if err = db.Ping(); err != nil {
		return fmt.Errorf("error connecting to the database: %v", err)
	}
	return applyMigrations()
}

func applyMigrations() error {
	migration, err := os.ReadFile("migrations/001_create_users_table.sql")
	if err != nil {
		return fmt.Errorf("error reading migration file: %v", err)
	}
	_, err = db.Exec(string(migration))
	if err != nil {
		return fmt.Errorf("error applying migration: %v", err)
	}
	log.Println("Database migrations applied successfully")
	return nil
}

func generateToken(userID int, username, userType string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id":   userID,
		"username":  username,
		"user_type": userType,
		"exp":       time.Now().Add(time.Hour * 12).Unix(),
		"iat":       time.Now().Unix(),
	})
	return token.SignedString([]byte(os.Getenv("JWT_SECRET")))
}

func registerCustomerHandler(c *gin.Context) {
	var req RegisterCustomerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{Success: false, Message: "Invalid request body"})
		return
	}
	customerID := fmt.Sprintf("CUST-%d", time.Now().Unix())
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Message: "Error processing password"})
		return
	}
	tx, err := db.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Message: "Error creating user"})
		return
	}
	defer tx.Rollback()
	var userID int
	err = tx.QueryRow(
		`INSERT INTO users (username, password, user_type) VALUES ($1, $2, $3) RETURNING id`,
		req.Username,
		string(hashedPassword),
		UserTypeCustomer,
	).Scan(&userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Message: "Error creating user"})
		return
	}
	_, err = tx.Exec(
		`INSERT INTO customer_details (user_id, name, customer_id) VALUES ($1, $2, $3)`,
		userID,
		req.Name,
		customerID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Message: "Error creating customer details"})
		return
	}
	if err = tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Message: "Error completing registration"})
		return
	}
	c.JSON(http.StatusOK, Response{Success: true, Data: nil})
}

func registerCompanyHandler(c *gin.Context) {
	var req RegisterCompanyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{Success: false, Message: "Invalid request body"})
		return
	}
	companyID := fmt.Sprintf("COMP-%d", time.Now().Unix())
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Message: "Error processing password"})
		return
	}
	tx, err := db.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Message: "Error creating user"})
		return
	}
	defer tx.Rollback()
	var userID int
	err = tx.QueryRow(
		`INSERT INTO users (username, password, user_type) VALUES ($1, $2, $3) RETURNING id`,
		req.Username,
		string(hashedPassword),
		UserTypeCompany,
	).Scan(&userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Message: "Error creating user"})
		return
	}
	_, err = tx.Exec(
		`INSERT INTO company_details (user_id, company_name, company_id) VALUES ($1, $2, $3)`,
		userID,
		req.CompanyName,
		companyID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Message: "Error creating company details"})
		return
	}
	if err = tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Message: "Error completing registration"})
		return
	}
	c.JSON(http.StatusOK, Response{Success: true, Data: nil})
}

func loginHandler(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{Success: false, Message: "Invalid request body"})
		return
	}
	var user BaseUser
	err := db.QueryRow(
		"SELECT id, username, password, user_type FROM users WHERE username = $1",
		req.Username,
	).Scan(&user.ID, &user.Username, &user.Password, &user.UserType)
	if err != nil {
		c.JSON(http.StatusUnauthorized, Response{Success: false, Message: "Invalid credentials"})
		return
	}
	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password))
	if err != nil {
		c.JSON(http.StatusUnauthorized, Response{Success: false, Message: "Invalid credentials"})
		return
	}
	token, err := generateToken(user.ID, user.Username, user.UserType)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Message: "Error generating token"})
		return
	}
	c.JSON(http.StatusOK, Response{Success: true, Data: Token{SignedToken: token}})
}

func init() {
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file not found")
	}
	if err := initDB(); err != nil {
		log.Fatal("Failed to initialize database:", err)
	}
}

func main() {
	r := gin.Default()
	r.POST("/register/customer", registerCustomerHandler)
	r.POST("/register/company", registerCompanyHandler)
	r.POST("/login", loginHandler)
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Server starting on port %s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatal(err)
	}
}
