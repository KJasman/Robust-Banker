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
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

// -----------------------------------------------------------------------------
// The same domain models used by order-service/matching-service
// -----------------------------------------------------------------------------

type NullString struct {
	String string
	Valid  bool
}

func (ns NullString) MarshalJSON() ([]byte, error) {
	if !ns.Valid {
		return []byte("null"), nil
	}

	return []byte(fmt.Sprintf(`"%s"`, ns.String)), nil
}

// Order struct exactly as in your code
type Stock struct {
	StockID     int       `json:"stock_id"`
	StockName   string    `json:"stock_name"`
	MarketPrice float64   `json:"market_price"`
	Quantity    int       `json:"quantity"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// The main Order struct
type Order struct {
	StockID         int        `json:"stock_id"`
	StockTxID       string     `json:"stock_tx_id"`
	ParentStockTxID string     `json:"parent_stock_tx_id"`
	WalletTxID      NullString `json:"wallet_tx_id"`
	UserID          int        `json:"user_id"`
	StockData       Stock      `json:"stock_data"`
	OrderType       string     `json:"order_type"`
	IsBuy           bool       `json:"is_buy"`
	Quantity        int        `json:"quantity"`
	Price           float64    `json:"price"`
	Status          string     `json:"order_status"`
	Created         time.Time  `json:"created"`
}

// Generic response
type Response struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Message string      `json:"message,omitempty"`
}

// -----------------------------------------------------------------------------
// Global DB handle
// -----------------------------------------------------------------------------

var db *sql.DB

func main() {
	// Load env, connect to DB
	_ = godotenv.Load()
	dsn := os.Getenv("TIMESCALE_DSN")
	if dsn == "" {
		// Example DSN
		dsn = "postgresql://postgres@timescale:5432/stockdb?sslmode=disable"
	}
	var err error
	db, err = sql.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("Failed to open DB: %v", err)
	}
	if err = db.Ping(); err != nil {
		log.Fatalf("DB ping error: %v", err)
	}

	// Setup Gin
	r := gin.Default()

	// Health check
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "order-history"})
	})

	// (1) Endpoint for the matching-service or order-service to record transactions
	r.POST("/internal/recordStockTransaction", recordTransactionHandler)

	// (2) Endpoint for users (via gateway) to fetch their transactions
	r.GET("/getStockTransactions", getStockTransactionsHandler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8082"
	}
	log.Printf("order-history service listening on :%s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatal(err)
	}
}

// -----------------------------------------------------------------------------
//  1. recordTransactionHandler
//     Called by matching‐service (or order‐service) to insert final or partial records
//
// -----------------------------------------------------------------------------
func recordTransactionHandler(c *gin.Context) {
	var o Order
	if err := c.ShouldBindJSON(&o); err != nil {
		c.JSON(http.StatusBadRequest, Response{Success: false, Message: "Invalid JSON body"})
		return
	}

	// Decide which timestamp to store. If o.Created is zero, use now.
	tstamp := o.Created
	if tstamp.IsZero() {
		tstamp = time.Now()
	}

	// Convert NullString to a normal string or nil
	var wtx interface{}
	if o.WalletTxID.Valid {
		wtx = o.WalletTxID.String
	} else {
		wtx = nil
	}

	// Insert into Timescale
	_, err := db.Exec(`
        INSERT INTO stock_transactions (
            stock_tx_id, parent_stock_tx_id, wallet_tx_id,
            stock_id, user_id, order_status, is_buy, order_type,
            stock_price, quantity, time_stamp
        )
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
    `,
		o.StockTxID,
		nullIfEmpty(o.ParentStockTxID),
		wtx,
		o.StockID,
		o.UserID,
		o.Status,
		o.IsBuy,
		o.OrderType,
		o.Price,
		o.Quantity,
		tstamp,
	)
	if err != nil {
		log.Println("DB insert error:", err)
		c.JSON(http.StatusInternalServerError, Response{
			Success: false,
			Message: "Failed to insert into stock_transactions",
		})
		return
	}

	// Return success
	c.JSON(http.StatusOK, Response{Success: true, Message: "Transaction recorded"})
}

// -----------------------------------------------------------------------------
//  2. getStockTransactionsHandler
//     Called by the user (customer or company) via the gateway. We show all orders
//     that belong to them (UserID), sorted by time ascending.
//
// -----------------------------------------------------------------------------
func getStockTransactionsHandler(c *gin.Context) {
	userIDStr := c.GetHeader("X-User-ID")
	if userIDStr == "" {
		c.JSON(http.StatusUnauthorized, Response{
			Success: false,
			Message: "Missing X-User-ID",
		})
		return
	}
	userID, err := strconv.Atoi(userIDStr)
	if err != nil || userID <= 0 {
		c.JSON(http.StatusUnauthorized, Response{
			Success: false,
			Message: "Invalid X-User-ID",
		})
		return
	}

	// Retrieve all rows for that user.
	rows, err := db.Query(`
        SELECT stock_tx_id, parent_stock_tx_id, wallet_tx_id,
               stock_id, user_id, order_status, is_buy, order_type,
               stock_price, quantity, time_stamp
          FROM stock_transactions
         WHERE user_id = $1
         ORDER BY time_stamp ASC
    `, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Success: false,
			Message: "DB query error",
		})
		return
	}
	defer rows.Close()

	var out []Order
	for rows.Next() {
		var rec Order
		var ptx, wtx *string
		var tstamp time.Time
		if scanErr := rows.Scan(
			&rec.StockTxID,
			&ptx,
			&wtx,
			&rec.StockID,
			&rec.UserID,
			&rec.Status,
			&rec.IsBuy,
			&rec.OrderType,
			&rec.Price,
			&rec.Quantity,
			&tstamp,
		); scanErr != nil {
			log.Println("Scan error:", scanErr)
			continue
		}
		if ptx != nil {
			rec.ParentStockTxID = *ptx
		}
		if wtx != nil {
			rec.WalletTxID = NullString{String: *wtx, Valid: true}
		}
		rec.Created = tstamp
		out = append(out, rec)
	}

	c.JSON(http.StatusOK, Response{Success: true, Data: out})
}

// -----------------------------------------------------------------------------
// Helpers
// -----------------------------------------------------------------------------

func nullIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
