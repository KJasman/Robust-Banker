package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gocql/gocql"
	"github.com/joho/godotenv"
)

// NullString is a custom type to store possibly-NULL strings from Cassandra
// and produce "null" in JSON if Valid=false.
type NullString struct {
	String string
	Valid  bool
}

func (ns NullString) MarshalJSON() ([]byte, error) {
	if !ns.Valid {
		return []byte("null"), nil
	}
	return json.Marshal(ns.String)
}

func (ns *NullString) ScanCQL(value interface{}) {
	if value == nil {
		ns.String, ns.Valid = "", false
	} else {
		ns.String = value.(string)
		ns.Valid = true
	}
}

// Order type with some fields as NullString so Cassandra can store null
type Order struct {
	StockID         int        `json:"stock_id"`
	StockTxID       string     `json:"stock_tx_id"`
	ParentStockTxID NullString `json:"parent_stock_tx_id"`
	WalletTxID      NullString `json:"wallet_tx_id"`
	UserID          int        `json:"user_id"`
	StockData       Stock      `json:"stock_data"`
	OrderType       string     `json:"order_type"`
	IsBuy           bool       `json:"is_buy"`
	Quantity        int        `json:"quantity"`
	Price           float64    `json:"price"`
	Status          NullString `json:"order_status"`
	Created         time.Time  `json:"created"`
}

type Stock struct {
	StockID     int       `json:"stock_id"`
	StockName   string    `json:"stock_name"`
	MarketPrice float64   `json:"market_price"`
	Quantity    int       `json:"quantity"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Response struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data"`
}

type Error struct {
	Message string `json:"message"`
}

var (
	ordersSession *gocql.Session
	stocksSession *gocql.Session
)

// Just a test to confirm we can query from the orders keyspace
func testCassandraConnection() {
	var count int
	err := ordersSession.Query("SELECT COUNT(*) FROM orders_keyspace.market_buy").Scan(&count)
	if err != nil {
		fmt.Println("❌ Cassandra Connection Issue:", err)
	} else {
		fmt.Println("✅ Cassandra is working! Orders Count (market_buy):", count)
	}
}

// initDB creates/ensures both keyspaces exist, then opens two sessions,
// one pointing to the stocks keyspace and another to the orders keyspace.
func initDB() error {
	cluster := gocql.NewCluster(os.Getenv("CASSANDRA_DB_HOST"))

	portStr := os.Getenv("CASSANDRA_DB_PORT")
	if portStr == "" {
		portStr = "9042"
	}
	portNum, _ := strconv.Atoi(portStr)
	cluster.Port = portNum

	// We temporarily connect without specifying a keyspace, so we can CREATE it if needed.
	cluster.Keyspace = ""
	cluster.Authenticator = gocql.PasswordAuthenticator{
		Username: os.Getenv("DB_USER"),
		Password: os.Getenv("DB_PASSWORD"),
	}
	cluster.Consistency = gocql.One

	tempSession, err := cluster.CreateSession()
	if err != nil {
		return fmt.Errorf("❌ error connecting to Cassandra initially: %v", err)
	}
	defer tempSession.Close()

	// Ensure orders_keyspace
	err = tempSession.Query(`
        CREATE KEYSPACE IF NOT EXISTS orders_keyspace
        WITH replication = {'class': 'SimpleStrategy', 'replication_factor': 1}
    `).Exec()
	if err != nil {
		return fmt.Errorf("❌ error creating orders_keyspace: %v", err)
	}

	// Ensure stocks_keyspace
	err = tempSession.Query(`
        CREATE KEYSPACE IF NOT EXISTS stocks_keyspace
        WITH replication = {'class': 'NetworkTopologyStrategy', 'datacenter1': 1}
    `).Exec()
	if err != nil {
		return fmt.Errorf("❌ error creating stocks_keyspace: %v", err)
	}

	fmt.Println("✅ Keyspaces verified or created successfully!")

	// Now connect for the stocks keyspace
	stocksCluster := *cluster
	stocksCluster.Keyspace = os.Getenv("CASSANDRA_DB_STOCKS_KEYSPACE") // typically "stocks_keyspace"
	stocksSession, err = stocksCluster.CreateSession()
	if err != nil {
		return fmt.Errorf("❌ error connecting to Cassandra stocks keyspace: %v", err)
	}
	fmt.Println("✅ Connected to stocks keyspace successfully!")

	// Connect for the orders keyspace
	ordersCluster := *cluster
	ordersCluster.Keyspace = os.Getenv("CASSANDRA_DB_ORDERS_KEYSPACE") // typically "orders_keyspace"
	ordersSession, err = ordersCluster.CreateSession()
	if err != nil {
		return fmt.Errorf("❌ error connecting to Cassandra orders keyspace: %v", err)
	}
	fmt.Println("✅ Connected to orders keyspace successfully!")

	return applyMigrations()
}

// applyMigrations runs the two .cql files against the correct sessions.
func applyMigrations() error {
	// 1) Migrate the orders keyspace tables
	csd1 := "migrations/001_active_order_table.cql"
	migration, err := os.ReadFile(csd1)
	if err != nil {
		return fmt.Errorf("error reading migration file %s: %v", csd1, err)
	}
	migrationQueries := strings.Split(string(migration), ";")
	for _, query := range migrationQueries {
		query = strings.TrimSpace(query)
		if query != "" {
			if err := ordersSession.Query(query).Exec(); err != nil {
				return fmt.Errorf("❌error applying migration %s: %v", csd1, err)
			}
		}
	}
	log.Printf("✅ Migration %s applied successfully\n", csd1)

	// 2) Migrate the stocks keyspace tables
	csd2 := "migrations/002_stock_table.cql"
	migration, err = os.ReadFile(csd2)
	if err != nil {
		return fmt.Errorf("error reading migration file %s: %v", csd2, err)
	}
	migrationQueries = strings.Split(string(migration), ";")
	for _, query := range migrationQueries {
		query = strings.TrimSpace(query)
		if query != "" {
			if err := stocksSession.Query(query).Exec(); err != nil {
				return fmt.Errorf("❌error applying migration %s: %v", csd2, err)
			}
		}
	}

	// Just to test we can query from the orders keyspace:
	testCassandraConnection()
	return nil
}

func init() {
	// Load local .env if present
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file not found (this may be OK if running in container)")
	}
	// Initialize DB connections + migrations
	if err := initDB(); err != nil {
		log.Fatal("Failed to initialize databases:", err)
	}
}

// ----------------------------------------------------
// Gin helper: checkAuthorization
// ----------------------------------------------------
func checkAuthorization(c *gin.Context) int {
	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, Response{
			Success: false,
			Data:    Error{Message: "Unauthorized: missing X-User-ID"},
		})
		c.Abort()
		return -1
	}
	userIDInt, err := strconv.Atoi(userID)
	if err != nil {
		c.JSON(http.StatusUnauthorized, Response{
			Success: false,
			Data:    Error{Message: "Invalid User ID"},
		})
		c.Abort()
		return -1
	}
	return userIDInt
}

// func checkCompanyAuthorization(c *gin.Context) bool {
// 	userType := c.GetHeader("X-User-Type")
// 	return (userType == "COMPANY")
// }

// ----------------------------------------------------
// Create Stock (Company action)
// ----------------------------------------------------
func createStock(c *gin.Context) {
	userID := checkAuthorization(c)
	if userID == -1 {
		return
	}
	// if !checkCompanyAuthorization(c) {
	// 	c.JSON(http.StatusUnauthorized, Response{
	// 		Success: false,
	// 		Data:    Error{Message: "Unauthorized: Only Company can perform this action"},
	// 	})
	// 	return
	// }

	var request Stock
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Success: false,
			Data:    Error{Message: "Invalid request body"},
		})
		return
	}

	// Check if the stock already exists by name
	var existingStockID int
	err := stocksSession.Query(`
        SELECT stock_id 
        FROM stocks_keyspace.stock_lookup 
        WHERE stock_name = ?
    `, request.StockName).Scan(&existingStockID)

	// If we found a stock_id AND it's nonzero, that means this name is taken
	if err == nil && existingStockID != 0 {
		c.JSON(http.StatusBadRequest, Response{
			Success: false,
			Data:    Error{Message: "Stock with this name already exists"},
		})
		return
	}

	// Generate new stock ID = totalStocks + 1
	var totalStocks int
	err = stocksSession.Query(`SELECT COUNT(*) FROM stocks_keyspace.stocks`).Scan(&totalStocks)
	if err != nil {
		msg := "Error fetching total stocks: " + err.Error()
		fmt.Println("❌", msg)
		c.JSON(http.StatusInternalServerError, Response{
			Success: false,
			Data:    Error{Message: msg},
		})
		return
	}
	request.StockID = totalStocks + 1
	request.MarketPrice = 0.0
	request.Quantity = 0
	request.UpdatedAt = time.Now()

	// Insert into stocks
	err = stocksSession.Query(`
        INSERT INTO stocks_keyspace.stocks (stock_id, stock_name, quantity, market_price, updated_at)
        VALUES (?, ?, ?, ?, ?)
    `, request.StockID, request.StockName, request.Quantity, request.MarketPrice, request.UpdatedAt).Exec()
	if err != nil {
		msg := "Error inserting stock: " + err.Error()
		fmt.Println("❌", msg)
		c.JSON(http.StatusInternalServerError, Response{
			Success: false,
			Data:    Error{Message: msg},
		})
		return
	}

	// Insert into stock_lookup
	err = stocksSession.Query(`
        INSERT INTO stocks_keyspace.stock_lookup (stock_name, stock_id)
        VALUES (?, ?)
    `, request.StockName, request.StockID).Exec()
	if err != nil {
		msg := "Error inserting stock into lookup: " + err.Error()
		fmt.Println("❌", msg)
		c.JSON(http.StatusInternalServerError, Response{
			Success: false,
			Data:    Error{Message: msg},
		})
		return
	}

	// Return the newly created stock ID
	type StockIDStruct struct {
		ID int `json:"stock_id"`
	}
	c.JSON(http.StatusOK, Response{Success: true, Data: StockIDStruct{ID: request.StockID}})
}

// ----------------------------------------------------
// Add Stock To User (Company action) - basically update stock quantity
// ----------------------------------------------------
func addStockToUser(c *gin.Context) {
	userID := checkAuthorization(c)
	if userID == -1 {
		return
	}
	// if !checkCompanyAuthorization(c) {
	// 	c.JSON(http.StatusUnauthorized, Response{
	// 		Success: false,
	// 		Data:    Error{Message: "Unauthorized: Only Company can perform this action"},
	// 	})
	// 	return
	// }

	var request Stock
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Success: false,
			Data:    Error{Message: "Invalid request body"},
		})
		return
	}

	var existingQty int
	err := stocksSession.Query(`
        SELECT quantity 
        FROM stocks_keyspace.stocks 
        WHERE stock_id = ?
    `, request.StockID).Scan(&existingQty)

	if err != nil {
		msg := "Invalid stock ID or error reading quantity: " + err.Error()
		fmt.Println("❌", msg)
		c.JSON(http.StatusBadRequest, Response{
			Success: false, Data: Error{Message: msg},
		})
		return
	}

	newQty := existingQty + request.Quantity
	updatedAt := time.Now()

	err = stocksSession.Query(`
        UPDATE stocks_keyspace.stocks 
        SET quantity = ?, updated_at = ?
        WHERE stock_id = ?
    `,
		newQty, updatedAt, request.StockID).Exec()

	if err != nil {
		msg := "Error updating stock quantity: " + err.Error()
		fmt.Println("❌", msg)
		c.JSON(http.StatusInternalServerError, Response{
			Success: false, Data: Error{Message: msg},
		})
		return
	}
	fmt.Println("✅ Stock quantity updated successfully")
	c.JSON(http.StatusOK, Response{Success: true, Data: nil})
}

// ----------------------------------------------------
// Place Stock Order (Customer action) => Market or Limit
// ----------------------------------------------------
func placeStockOrder(c *gin.Context) {
	userID := checkAuthorization(c)
	if userID == -1 {
		return
	}

	var request Order
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Success: false, Data: Error{Message: "Invalid request body"},
		})
		return
	}
	request.UserID = userID

	if request.Quantity <= 0 {
		c.JSON(http.StatusBadRequest, Response{
			Success: false, Data: Error{Message: "Invalid quantity"},
		})
		return
	}

	switch strings.ToUpper(request.OrderType) {
	case "MARKET":
		placeMarketOrder(request, c)
	case "LIMIT":
		placeLimitOrder(request, c)
	default:
		c.JSON(http.StatusBadRequest, Response{
			Success: false, Data: Error{Message: "Invalid order type (must be MARKET or LIMIT)"},
		})
	}
}

func placeMarketOrder(request Order, c *gin.Context) {
	if request.Price != 0 {
		c.JSON(http.StatusBadRequest, Response{
			Success: false, Data: Error{Message: "Market orders cannot have a price"},
		})
		return
	}
	stockTxID := gocql.TimeUUID()
	now := time.Now()

	var err error
	if request.IsBuy {
		// Insert into orders_keyspace.market_buy
		err = ordersSession.Query(`
            INSERT INTO orders_keyspace.market_buy
                (stock_id, stock_tx_id, parent_stock_tx_id, wallet_tx_id, 
                 user_id, order_type, is_buy, quantity, price, order_status, 
                 created_at, updated_at)
            VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
        `,
			request.StockID,
			stockTxID,
			nil,
			nil,
			request.UserID,
			"MARKET",
			true,
			request.Quantity,
			0.0,
			"IN_PROGRESS",
			now,
			now,
		).Exec()
	} else {
		// Insert into orders_keyspace.market_sell
		err = ordersSession.Query(`
            INSERT INTO orders_keyspace.market_sell
                (stock_id, stock_tx_id, parent_stock_tx_id, wallet_tx_id,
                 user_id, order_type, is_buy, quantity, price, order_status,
                 created_at, updated_at)
            VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
        `,
			request.StockID,
			stockTxID,
			nil,
			nil,
			request.UserID,
			"MARKET",
			false,
			request.Quantity,
			0.0,
			"IN_PROGRESS",
			now,
			now,
		).Exec()
	}

	if err != nil {
		msg := "Error placing MARKET order: " + err.Error()
		fmt.Println("❌", msg)
		c.JSON(http.StatusInternalServerError, Response{
			Success: false, Data: Error{Message: msg},
		})
		return
	}

	c.JSON(http.StatusOK, Response{Success: true, Data: nil})
}

func placeLimitOrder(request Order, c *gin.Context) {
	if request.Price <= 0 {
		c.JSON(http.StatusBadRequest, Response{
			Success: false, Data: Error{Message: "Invalid price for LIMIT order"},
		})
		return
	}
	stockTxID := gocql.TimeUUID()
	now := time.Now()

	var err error
	if request.IsBuy {
		// Insert into orders_keyspace.limit_buy
		err = ordersSession.Query(`
            INSERT INTO orders_keyspace.limit_buy
                (stock_id, stock_tx_id, parent_stock_tx_id, wallet_tx_id,
                 user_id, order_type, is_buy, quantity, price, order_status,
                 created_at, updated_at)
            VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
        `,
			request.StockID,
			stockTxID,
			nil,
			nil,
			request.UserID,
			"LIMIT",
			true,
			request.Quantity,
			request.Price,
			"IN_PROGRESS",
			now,
			now,
		).Exec()
	} else {
		// Insert into orders_keyspace.limit_sell
		err = ordersSession.Query(`
            INSERT INTO orders_keyspace.limit_sell
                (stock_id, stock_tx_id, parent_stock_tx_id, wallet_tx_id,
                 user_id, order_type, is_buy, quantity, price, order_status,
                 created_at, updated_at)
            VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
        `,
			request.StockID,
			stockTxID,
			nil,
			nil,
			request.UserID,
			"LIMIT",
			false,
			request.Quantity,
			request.Price,
			"IN_PROGRESS",
			now,
			now,
		).Exec()
	}

	if err != nil {
		msg := "Error placing LIMIT order: " + err.Error()
		fmt.Println("❌", msg)
		c.JSON(http.StatusInternalServerError, Response{
			Success: false, Data: Error{Message: msg},
		})
		return
	}

	c.JSON(http.StatusOK, Response{Success: true, Data: nil})
}

// ----------------------------------------------------
// Cancel Stock Transaction
// ----------------------------------------------------
func cancelStockTransaction(c *gin.Context) {
	userID := checkAuthorization(c)
	if userID == -1 {
		return
	}

	var req struct {
		StockTxID int `json:"stock_tx_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Success: false,
			Data:    Error{Message: "Invalid request body"},
		})
		return
	}

	// For now, we simply respond success
	fmt.Println("Cancelling stock transaction with ID:", req.StockTxID, "for user:", userID)
	c.JSON(http.StatusOK, Response{Success: true, Data: nil})
}

// ----------------------------------------------------
// main() - Start the Gin server
// ----------------------------------------------------
func main() {
	r := gin.Default()

	// Routes
	r.POST("/engine/placeStockOrder", placeStockOrder)
	r.POST("/engine/cancelStockTransaction", cancelStockTransaction)
	r.POST("/setup/createStock", createStock)
	r.POST("/setup/addStockToUser", addStockToUser)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}
	log.Printf("Order service starting on port %s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatal(err)
	}
}
