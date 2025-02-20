package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

type WalletTransaction struct { // debit total amount of newly bought stock (completed transaction)
	WalletID   string    `json:"wallet_id"`
	WalletTxID string    `json:"wallet_tx_id"`
	StockTxID  string    `json:"stock_tx_id"`
	IsDebit    bool      `json:"is_debit"`
	Amount     float64   `json:"amount"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type StockTransaction struct { // all active and inactive transactions
	StockTxID       string    `json:"stock_tx_id"`
	ParentStockTxID string    `json:"parent_stock_tx_id"`
	StockID         string    `json:"stock_id"`
	WalletTxID      string    `json:"wallet_tx_id"` // if completed transaction, it will be NOT NULL, referencing the wallet_tx_id of debit/withdraw transaction
	Status          string    `json:"order_status"`
	IsBuy           bool      `json:"is_buy"`
	OrderType       string    `json:"order_type"`
	StockPrice      float64   `json:"stock_price"`
	Quantity        int       `json:"quantity"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type Wallet struct {
	UserID     int     `json:"user_id"`
	WalletID   string  `json:"wallet_id"`
	Balance    float64 `json:"balance"`
	StockOwned Stock   `json:"stock_owned"`
}

type Stock struct { // Stock porfolio, current stock owned, if a sell order is placed, it will be removed from the stock portfolio
	StockID   string    `json:"stock_id"`
	StockName string    `json:"stock_name"`
	Quantity  int       `json:"quantity_owned"`
	UpdatedAt time.Time `json:"stock_updated_at"`
}

type Error struct {
	Message string `json:"message"`
}

type Response struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data"`
}

var (
	portfolioSession *sql.DB
)

func initDB() error {
	var cr = "postgresql://root@cockroach-db:26257/?sslmode=disable"
	db, err := sql.Open("postgres", cr)
	if err != nil {
		return fmt.Errorf("failed to open database: %v", err)
	}
	defer db.Close()

	_, err = db.Exec(`CREATE DATABASE IF NOT EXISTS "portfolio-db";`)
	if err != nil {
		return fmt.Errorf("❌ error creating database: %v", err)
	}
	fmt.Println("✅ Ensured 'portfolio-db' exists!")

	cr = "postgresql://root@cockroach-db:26257/portfolio-db?sslmode=disable"
	db, err = sql.Open("postgres", cr)
	if err != nil {
		return fmt.Errorf("failed to connect to portfolio-db: %v", err)
	}
	if err = db.Ping(); err != nil {
		db.Close()
		return fmt.Errorf("❌error connecting to the COCKROACH database: %v", err)
	}
	fmt.Println("✅Connected to 'portfolio-db' successfully!")
	portfolioSession = db

	return applyMigrations()
}

func applyMigrations() error {
	if portfolioSession == nil {
		return fmt.Errorf("❌ portfolioSession is nil, database connection not established")
	}

	migrations := map[*sql.DB]string{
		portfolioSession: "migrations/portfolio_table.sql",
	}

	for db, filePath := range migrations {
		migration, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("❌ error reading migration file %s: %v", filePath, err)
		}

		_, err = db.Exec(string(migration))
		if err != nil {
			return fmt.Errorf("❌error applying migration %s: %v", filePath, err)
		}

		log.Printf("✅Migration %s applied successfully\n", filePath)
	}
	return nil
}

func init() {
	// Load environment variables from .env file
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	// Initialize database connection
	if err := initDB(); err != nil {
		log.Fatal("❌Failed to initialize databases:", err)
	}
}

// TODO: update company stock own by listen to addStockToUser API of order-service

func checkAuthorization(c *gin.Context) int {
	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, Response{Success: false, Data: Error{Message: "Unauthorized"}})
		c.Abort()
		return -1
	}
	userIDInt, err := strconv.Atoi(userID)
	if err != nil {
		c.JSON(http.StatusUnauthorized, Response{Success: false, Data: Error{Message: "Invalid User ID"}})
		c.Abort()
		return -1
	}
	return userIDInt
}

func checkCompanyAuthorization(c *gin.Context) int {
	userType := c.GetHeader("X-User-Type")
	if userType != "COMPANY" && userType == "CUSTOMER" {
		c.JSON(http.StatusUnauthorized, Response{Success: false, Data: Error{Message: "Unauthorized: Only Company can perform this action"}})
		return 0
	}
	return 1
}

func createWallet(userID int) (string, error) {
	walletID := uuid.New().String()
	balance := 0.0
	_, err := portfolioSession.Exec(
		`INSERT INTO wallet (wallet_id, user_id, balance) VALUES ($1, $2, $3)`,
		walletID, userID, balance)
	if err != nil {
		fmt.Println("❌ error creating wallet:", err)
		return "", err
	}
	return walletID, nil
}
func addMoneyToWallet(c *gin.Context) {
	userID := checkAuthorization(c)
	if userID == -1 {
		return
	}
	var request struct {
		Amount float64 `json:"amount"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}
	if request.Amount <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Amount must be greater than 0"})
		return
	}
	var walletID string
	err := portfolioSession.QueryRow(`SELECT wallet_id FROM wallet WHERE user_id = $1`, userID).Scan(&walletID)
	if err != nil {
		if err == sql.ErrNoRows {
			var createErr error
			walletID, createErr = createWallet(userID)
			if createErr != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create wallet"})
				return
			}
		}
	}

	_, err = portfolioSession.Exec(`UPDATE wallet SET balance = balance + $1 WHERE wallet_id = $2`, request.Amount, walletID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update wallet"})
		return
	}
	c.JSON(http.StatusOK, Response{Success: true, Data: nil})
}

func getWalletBalance(c *gin.Context) {
	userID := checkAuthorization(c)
	if userID == -1 {
		return
	}

	var balance float64
	err := portfolioSession.QueryRow(`SELECT balance FROM wallet WHERE user_id = $1`, userID).Scan(&balance)
	if err != nil {
		if err == sql.ErrNoRows {
			_, createErr := createWallet(userID)
			if createErr != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create wallet"})
				return
			}
		}
	}
	err = portfolioSession.QueryRow(`SELECT balance FROM wallet WHERE user_id = $1`, userID).Scan(&balance)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch wallet balance"})
		return
	}

	type Balance struct {
		Balance float64 `json:"balance"`
	}
	c.JSON(http.StatusOK, Response{Success: true, Data: Balance{Balance: balance}})
}

// func getStockPortfolio(c *gin.Context) { // Read from order-history service
// 	userID := checkAuthorization(c)
// 	if userID == -1 {
// 		return
// 	}
// 	companyID := checkCompanyAuthorization(c)
// 	if companyID == 0 {
// 		return
// 	}

// 	walletAddress := c.Query("walletAddress")
// 	if walletAddress == "" {
// 		c.JSON(400, gin.H{"error": "Wallet address is required"})
// 		return
// 	}
// }

func main() {
	// r := gin.Default()
	gin.SetMode(gin.ReleaseMode) // 🔥 Switch to release mode

	r := gin.New()                      // Creates a fresh Gin instance
	r.Use(gin.Logger(), gin.Recovery()) // Add middleware explicitly

	// r.POST("/api/v1/wallet/getStockPortfolio", getStockPortfolio)
	r.POST("/api/v1/wallet/addMoneyToWallet", addMoneyToWallet)
	r.GET("/api/v1/wallet/getWalletBalance", getWalletBalance)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8082"
	}

	log.Printf("Server starting on port %s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatal(err)
	}
}
