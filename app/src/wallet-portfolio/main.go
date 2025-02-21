package main

import (
	"database/sql"
	"encoding/json"
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

// -----------------------------------------------------------------------------
// Custom NullString type to marshal NULL in JSON as "null" instead of an object
// -----------------------------------------------------------------------------

type NullString struct {
	sql.NullString
}

// MarshalJSON ensures if Valid == false, we get null in JSON.
func (ns NullString) MarshalJSON() ([]byte, error) {
	if !ns.Valid {
		return []byte("null"), nil
	}
	return json.Marshal(ns.String)
}

// -----------------------------------------------------------------------------
// Data Models
// -----------------------------------------------------------------------------

type WalletTransaction struct {
	WalletTxID string     `json:"wallet_tx_id"`
	StockTxID  NullString `json:"stock_tx_id"` // can be NULL
	IsDebit    bool       `json:"is_debit"`
	Amount     float64    `json:"amount"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

type StockPortfolioItem struct {
	StockID       int       `json:"stock_id"`
	QuantityOwned int       `json:"quantity_owned"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type Response struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data"`
	Message string      `json:"message,omitempty"`
}

// -----------------------------------------------------------------------------
// Globals & DB Initialization
// -----------------------------------------------------------------------------

var portfolioDB *sql.DB

func initDB() error {
	dsn := "postgresql://root@cockroach-db:26257/?sslmode=disable"
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return fmt.Errorf("error opening DB: %v", err)
	}
	defer db.Close()

	_, err = db.Exec(`CREATE DATABASE IF NOT EXISTS "portfolio-db";`)
	if err != nil {
		return fmt.Errorf("error creating 'portfolio-db': %v", err)
	}

	dsn = "postgresql://root@cockroach-db:26257/portfolio-db?sslmode=disable"
	portfolioDB, err = sql.Open("postgres", dsn)
	if err != nil {
		return fmt.Errorf("error connecting to 'portfolio-db': %v", err)
	}
	if err = portfolioDB.Ping(); err != nil {
		portfolioDB.Close()
		return fmt.Errorf("error pinging 'portfolio-db': %v", err)
	}

	return applyMigrations(portfolioDB)
}

func applyMigrations(db *sql.DB) error {
	content, err := os.ReadFile("migrations/portfolio_table.sql")
	if err != nil {
		return fmt.Errorf("failed reading migration file: %w", err)
	}
	if _, err := db.Exec(string(content)); err != nil {
		return fmt.Errorf("failed to apply migrations: %w", err)
	}
	log.Println("✅ Migrations applied successfully.")
	return nil
}

func init() {
	_ = godotenv.Load()
	if err := initDB(); err != nil {
		log.Fatalf("Could not init DB: %v", err)
	}
}

// -----------------------------------------------------------------------------
// Helpers
// -----------------------------------------------------------------------------

func checkAuthorization(c *gin.Context) int {
	userIDStr := c.GetHeader("X-User-ID")
	if userIDStr == "" {
		c.JSON(http.StatusUnauthorized, Response{
			Success: false,
			Data:    nil,
			Message: "Unauthorized (missing X-User-ID header)",
		})
		c.Abort()
		return -1
	}
	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		c.JSON(http.StatusUnauthorized, Response{
			Success: false,
			Data:    nil,
			Message: "Invalid X-User-ID header",
		})
		c.Abort()
		return -1
	}
	return userID
}

func createWalletIfNotExists(userID int) (string, error) {
	var walletID string
	err := portfolioDB.QueryRow(`SELECT wallet_id FROM wallet WHERE user_id=$1`, userID).Scan(&walletID)
	if err == sql.ErrNoRows {
		walletID = uuid.NewString()
		_, err = portfolioDB.Exec(`
			INSERT INTO wallet (wallet_id, user_id, balance) VALUES ($1, $2, 0)
		`, walletID, userID)
		if err != nil {
			return "", err
		}
		return walletID, nil
	} else if err != nil {
		return "", err
	}
	return walletID, nil
}

// -----------------------------------------------------------------------------
// Handlers
// -----------------------------------------------------------------------------

func addMoneyHandler(c *gin.Context) {
	userID := checkAuthorization(c)
	if userID == -1 {
		return
	}
	var req struct {
		Amount float64 `json:"amount"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{Success: false, Message: "Invalid request body"})
		return
	}
	if req.Amount <= 0 {
		c.JSON(http.StatusBadRequest, Response{Success: false, Message: "Amount must be > 0"})
		return
	}

	walletID, err := createWalletIfNotExists(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Message: "Failed to create/fetch wallet"})
		return
	}

	tx, err := portfolioDB.BeginTx(c, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Message: "DB transaction error"})
		return
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(c,
		`UPDATE wallet
         SET balance = balance + $1,
             updated_at = current_timestamp
         WHERE wallet_id = $2`,
		req.Amount, walletID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Message: "Failed to update balance"})
		return
	}

	walletTxID := uuid.NewString()
	_, err = tx.ExecContext(c,
		`INSERT INTO wallet_transactions (wallet_tx_id, wallet_id, is_debit, amount)
         VALUES ($1, $2, false, $3)`,
		walletTxID, walletID, req.Amount)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Message: "Failed to log transaction"})
		return
	}

	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Message: "Failed to commit transaction"})
		return
	}

	c.JSON(http.StatusOK, Response{Success: true, Data: nil})
}

func getWalletBalanceHandler(c *gin.Context) {
	userID := checkAuthorization(c)
	if userID == -1 {
		return
	}

	walletID, err := createWalletIfNotExists(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Message: "Failed to create/fetch wallet"})
		return
	}

	var balance float64
	err = portfolioDB.QueryRowContext(c,
		`SELECT balance FROM wallet WHERE wallet_id=$1`, walletID).Scan(&balance)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Message: "Error reading balance"})
		return
	}

	type Bal struct {
		Balance float64 `json:"balance"`
	}
	c.JSON(http.StatusOK, Response{Success: true, Data: Bal{Balance: balance}})
}

func getWalletTransactionsHandler(c *gin.Context) {
	userID := checkAuthorization(c)
	if userID == -1 {
		return
	}

	walletID, err := createWalletIfNotExists(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Success: false, Message: "Failed to create/fetch wallet",
		})
		return
	}

	rows, err := portfolioDB.QueryContext(c,
		`SELECT wallet_tx_id, stock_tx_id, is_debit, amount, updated_at
         FROM wallet_transactions
         WHERE wallet_id=$1
         ORDER BY updated_at DESC`, walletID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Success: false, Message: "Error querying transactions",
		})
		return
	}
	defer rows.Close()

	var txs []WalletTransaction
	for rows.Next() {
		var t WalletTransaction
		// stock_tx_id can be NULL, so it goes into t.StockTxID (which is NullString)
		if scanErr := rows.Scan(&t.WalletTxID, &t.StockTxID, &t.IsDebit, &t.Amount, &t.UpdatedAt); scanErr != nil {
			c.JSON(http.StatusInternalServerError, Response{
				Success: false, Message: "Error scanning transactions",
			})
			return
		}
		txs = append(txs, t)
	}
	c.JSON(http.StatusOK, Response{Success: true, Data: txs})
}

func getStockPortfolioHandler(c *gin.Context) {
	userID := checkAuthorization(c)
	if userID == -1 {
		return
	}

	walletID, err := createWalletIfNotExists(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Success: false, Message: "Failed to create/fetch wallet",
		})
		return
	}

	rows, err := portfolioDB.QueryContext(c,
		`SELECT stock_id, quantity_owned, updated_at
		 FROM stock_portfolio
		 WHERE wallet_id=$1
		 ORDER BY stock_id ASC`, walletID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Success: false, Message: "Error querying portfolio",
		})
		return
	}
	defer rows.Close()

	var items []StockPortfolioItem
	for rows.Next() {
		var spi StockPortfolioItem
		if scanErr := rows.Scan(&spi.StockID, &spi.QuantityOwned, &spi.UpdatedAt); scanErr != nil {
			c.JSON(http.StatusInternalServerError, Response{
				Success: false, Message: "Error scanning portfolio row",
			})
			return
		}
		items = append(items, spi)
	}
	c.JSON(http.StatusOK, Response{Success: true, Data: items})
}

func main() {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	txGroup := r.Group("/api/v1/transaction")
	{
		txGroup.POST("/addMoneyToWallet", addMoneyHandler)
		txGroup.GET("/getWalletBalance", getWalletBalanceHandler)
		txGroup.GET("/getWalletTransactions", getWalletTransactionsHandler)
		txGroup.GET("/getStockPortfolio", getStockPortfolioHandler)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8083"
	}
	log.Printf("Wallet-Portfolio service listening on port %s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatal(err)
	}
}
