package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	// CockroachDB
	// Web framework
	"github.com/gin-gonic/gin"
	"github.com/gocql/gocql"

	// Cassandra driver
	// PostgreSQL (TimescaleDB)
	"github.com/joho/godotenv" // Load environment variables
	_ "github.com/lib/pq"      // PostgreSQL driver
)

type Order struct {
	StockID         int       `json:"stock_id"`
	StockTxID       string    `json:"stock_tx_id"`
	ParentStockTxID string    `json:"parent_stock_tx_id"`
	WalletTxID      string    `json:"wallet_tx_id"`
	UserID          int       `json:"user_id"`
	StockData       Stock     `json:"stock_data"`
	OrderType       string    `json:"order_type"`
	IsBuy           bool      `json:"is_buy"`
	Quantity        int       `json:"quantity"`
	Price           float64   `json:"price"`
	Status          string    `json:"order_status"`
	Created         time.Time `json:"created"`
}

type Stock struct {
	StockID     int       `json:"stock_id"`
	StockName   string    `json:"stock_name"`
	MarketPrice float64   `json:"market_price"`
	Quantity    int       `json:"quantity"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type User struct {
	UserID  int     `json:"user_id"`
	Balance float64 `json:"balance"`
}

// type Wallet struct {
// 	UserID  int     `json:"user_id"`
// 	Balance float64 `json:"balance"`
// }

type Response struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data"`
}

type Error struct {
	Message string `json:"message"`
}

type CancelRequest struct {
	StockTxID int `json:"stock_tx_id"`
}

var (
	ordersSession *gocql.Session
	stocksSession *gocql.Session
)

func testCassandraConnection() {
	var count int
	err := ordersSession.Query("SELECT COUNT(*) FROM orders_keyspace.market_buy").Scan(&count)
	if err != nil {
		fmt.Println("❌ Cassandra Connection Issue:", err)
	} else {
		fmt.Println("✅ Cassandra is working! Orders Count:", count)
	}
}

func initDB() error {
	cluster := gocql.NewCluster(os.Getenv("CASSANDRA_DB_HOST"))
	cluster.Port, _ = strconv.Atoi(os.Getenv("CASSANDRA_DB_PORT"))
	cluster.Keyspace = os.Getenv("CASSANDRA_DB_KEYSPACE")
	cluster.Authenticator = gocql.PasswordAuthenticator{
		Username: os.Getenv("DB_USER"),
		Password: os.Getenv("DB_PASSWORD"),
	}
	cluster.Consistency = gocql.One

	tempSession, err := cluster.CreateSession()
	if err != nil {
		return fmt.Errorf("❌ error connecting to Cassandra: %v", err)
	}
	defer tempSession.Close()

	// Ensure orders_keyspace exists
	err = tempSession.Query(`
        CREATE KEYSPACE IF NOT EXISTS orders_keyspace 
        WITH replication = {'class': 'SimpleStrategy', 'replication_factor': 1}
    `).Exec()
	if err != nil {
		return fmt.Errorf("❌ error creating orders_keyspace: %v", err)
	}

	// Ensure stocks_keyspace exists
	err = tempSession.Query(`
        CREATE KEYSPACE IF NOT EXISTS stocks_keyspace 
        WITH replication = {'class': 'NetworkTopologyStrategy', 'datacenter1': 3}
    `).Exec()
	if err != nil {
		return fmt.Errorf("❌ error creating stocks_keyspace: %v", err)
	}

	fmt.Println("✅ Keyspaces verified or created successfully!")

	cluster.Keyspace = os.Getenv("CASSANDRA_DB_STOCKS_KEYSPACE")
	stocksSession, err = cluster.CreateSession()
	if err != nil {
		return fmt.Errorf("❌ error connecting to Cassandra stocks keyspace: %v", err)
	}
	fmt.Println("✅ Connected to stocks keyspace successfully!")

	// Connect to orders keyspace
	cluster.Keyspace = os.Getenv("CASSANDRA_DB_ORDERS_KEYSPACE")
	ordersSession, err = cluster.CreateSession()
	if err != nil {
		return fmt.Errorf("❌ error connecting to Cassandra orders keyspace: %v", err)
	}
	fmt.Println("✅ Connected to orders keyspace successfully!")

	return applyMigrations()
}

func applyMigrations() error {
	csd1 := "migrations/001_active_order_table.cql"
	migration, err := os.ReadFile(csd1)
	if err != nil {
		return fmt.Errorf("error reading migration file %s: %v", csd1, err)
	}

	migrationQueries := strings.Split(string(migration), ";")
	for _, query := range migrationQueries {
		if query != "" {
			err := ordersSession.Query(query).Exec()
			if err != nil {
				return fmt.Errorf("❌error applying migration %s: %v", csd1, err)
			}
		}
	}
	log.Printf("✅Migration %s applied successfully\n", csd1)

	csd2 := "migrations/002_stock_table.cql"
	migration, err = os.ReadFile(csd2)
	if err != nil {
		return fmt.Errorf("error reading migration file %s: %v", csd2, err)
	}

	migrationQueries = strings.Split(string(migration), ";")
	for _, query := range migrationQueries {
		if query != "" {
			err := stocksSession.Query(query).Exec()
			if err != nil {
				return fmt.Errorf("❌error applying migration %s: %v", csd2, err)
			}
		}
	}

	testCassandraConnection()
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
		log.Fatal("Failed to initialize databases:", err)
	}
}

// func getUserBalance(userID int) float64 {
// 	walletServiceURL := fmt.Sprintf("http://localhost:8000/api/v1/wallet/balance?user_id=%d", userID)

// 	connected, err := http.Get(walletServiceURL)
// 	if err != nil {
// 		log.Println("Error connecting Wallet Service: ", err)
// 		return 0
// 	}
// 	defer connected.Body.Close()

// 	var response struct {
// 		Success bool `json:"success"`
// 		Data struct {
// 			Wallet struct {
// 				Balance float64 `json:"balance"`
// 			} `json:"wallet"`
// 		} `json:"data"`
// 	}

// 	if err := json.NewDecoder(connected.Body).Decode(&response); err != nil {
// 		log.Println("Error decoding response:", err)
// 		return 0
// 	}

// 	if response.Success {
// 		return response.Data.Wallet.Balance
// 	}
// 	return 0
// }

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

func createStock(c *gin.Context) {
	is_authorized := checkAuthorization(c)
	if is_authorized == -1 {
		return
	}

	is_company := checkCompanyAuthorization(c)
	if is_company == 0 {
		return
	}
	var request Stock
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, Response{Success: false, Data: Error{Message: "Invalid request body"}})
		return
	}

	var existingStockID int
	err := stocksSession.Query("SELECT stock_id FROM stock_lookup WHERE stock_name = ?", request.StockName).Scan(&existingStockID)
	if err == nil && existingStockID != 0 { // stock already exists
		c.JSON(http.StatusBadRequest, Response{Success: false, Data: Error{Message: "Stock with this name already exists"}})
		return
	}

	var totalStocks int

	// TODO: Find better way to assign stockID
	err = stocksSession.Query("SELECT COUNT(*) FROM stocks").Scan(&totalStocks)
	if err != nil {
		fmt.Println("❌Error fetching total stocks:", err)
		c.JSON(http.StatusInternalServerError, Response{Success: false, Data: Error{Message: "Error fetching total stocks"}})
		return
	}
	// stockID := gocql.TimeUUID()
	request.StockID = totalStocks + 1
	request.MarketPrice = 0.0
	request.Quantity = 0

	currentTime := time.Now()
	request.UpdatedAt = currentTime

	fmt.Println("-------------------", request.StockName)

	err = stocksSession.Query(`
		INSERT INTO stocks (stock_id, stock_name, quantity, market_price, updated_at)
		VALUES (?, ?, ?, ?, ?)`,
		request.StockID,
		request.StockName,
		request.Quantity,
		request.MarketPrice,
		request.UpdatedAt,
	).Exec()

	if err != nil {
		fmt.Println("❌Error inserting stock into database:", err)
		c.JSON(http.StatusInternalServerError, Response{Success: false, Data: Error{Message: "Error inserting stock into database"}})
		return
	}

	err = stocksSession.Query(`
		INSERT INTO stock_lookup (stock_name, stock_id)
		VALUES (?, ?)`,
		request.StockName,
		request.StockID,
	).Exec()

	if err != nil {
		fmt.Println("❌Error inserting stock into lookup table:", err)
		c.JSON(http.StatusInternalServerError, Response{Success: false, Data: Error{Message: "Error inserting stock into lookup table"}})
		return
	}
	type StockID struct {
		ID int `json:"stock_id"`
	}

	c.JSON(http.StatusOK, Response{Success: true, Data: StockID{ID: request.StockID}})
}
func addStockToUser(c *gin.Context) {
	is_authorized := checkAuthorization(c)
	if is_authorized == -1 {
		return
	}

	is_company := checkCompanyAuthorization(c)
	if is_company == 0 {
		return
	}

	var request Stock
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, Response{Success: false, Data: Error{Message: "Invalid request body"}})
		return
	}

	var existingStockID int
	err := stocksSession.Query("SELECT quantity FROM stocks WHERE stock_id = ?", request.StockID).Scan(&existingStockID)
	if err != nil { // Stock ID does not exist
		fmt.Println("❌ Error fetching given ID:", err)
		c.JSON(http.StatusBadRequest, Response{Success: false, Data: Error{Message: "Invalid stock ID"}})
		return
	}

	request.Quantity = existingStockID + request.Quantity
	request.UpdatedAt = time.Now()

	err = stocksSession.Query(`
		UPDATE stocks_keyspace.stocks
		SET quantity = ?, updated_at = ?
		WHERE stock_id = ?`,
		request.Quantity, request.UpdatedAt,
		request.StockID).Exec()
	if err != nil {
		fmt.Println("❌ Error updating stock quantity:", err)
		c.JSON(http.StatusInternalServerError, Response{Success: false, Data: Error{Message: "Error updating stock quantity"}})
		return
	}

	fmt.Println("✅ Stock quantity updated successfully")
	c.JSON(http.StatusOK, Response{Success: true, Data: nil})
}

func placeOrderHandler(c *gin.Context) {
	userID := checkAuthorization(c)

	if userID == -1 {
		return
	}
	fmt.Println("✅ Authorized User ID:", userID)

	// Parse request body
	var request Order
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, Response{Success: false, Data: Error{Message: "Invalid request body"}})
		return
	}

	request.UserID = userID

	// Validate request
	if request.Quantity <= 0 {
		c.JSON(http.StatusBadRequest, Response{Success: false, Data: Error{Message: "Invalid quantity"}})
		return
	}
	// TODO:
	// Check if STOCK ID exists in stocks table
	// Check if QUANTITY is less than STOCK QUANTITY

	// balance := getUserBalance(request.UserID)
	// if request.Price > balance {
	// 	c.JSON(http.StatusBadRequest, gin.H{"error": "Insufficient balance"})
	// 	return
	// }

	if request.OrderType == "MARKET" {
		placeMarketOrder(request, c)
	} else if request.OrderType == "LIMIT" {
		placeLimitOrder(request, c)
	} else {
		c.JSON(http.StatusBadRequest, Response{Success: false, Data: Error{Message: "Invalid order type"}})
	}
}

func placeMarketOrder(request Order, c *gin.Context) {
	if request.Price != 0 {
		c.JSON(http.StatusBadRequest, Response{Success: false, Data: Error{Message: "Market orders cannot have a price"}})
		return
	}
	stockTxID := gocql.TimeUUID()
	request.Price = 0
	now := time.Now()
	fmt.Println("✅ Buy request: ", request.IsBuy, "Order ID: ", stockTxID, "Stock ID: ", request.StockID, "Quantity: ", request.Quantity)

	var err error
	if request.IsBuy {
		err = ordersSession.Query(`
            INSERT INTO orders_keyspace.market_buy 
            (stock_id, stock_tx_id, user_id, order_type, is_buy, quantity, price, order_status, created_at, updated_at)
            VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			request.StockID, stockTxID, request.UserID, "MARKET", 1, request.Quantity, request.Price, "IN_PROGRESS", now, now,
		).Exec()

	} else {
		err = ordersSession.Query(`
            INSERT INTO orders_keyspace.market_sell 
            (stock_id, stock_tx_id, user_id, order_type, is_buy, quantity, price, order_status, created_at, updated_at)
            VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			request.StockID, stockTxID, request.UserID, "MARKET", 0, request.Quantity, request.Price, "IN_PROGRESS", now, now,
		).Exec()
	}
	if err != nil {
		fmt.Println("❌ Cassandra Insert Error:", err)
		c.JSON(http.StatusInternalServerError, Response{Success: false, Data: Error{Message: "Error placing order"}})
		return
	}
	c.JSON(http.StatusOK, Response{Success: true, Data: nil})
}

func placeLimitOrder(request Order, c *gin.Context) {
	if request.Price <= 0 {
		c.JSON(http.StatusBadRequest, Response{Success: false, Data: Error{Message: "Invalid price"}})
		return
	}
	stockTxID := gocql.TimeUUID()
	now := time.Now()
	var err error
	if request.IsBuy {
		err = ordersSession.Query(`
            INSERT INTO orders_keyspace.limit_buy 
            (stock_id, stock_tx_id, user_id, order_type, is_buy, quantity, price, order_status, created_at, updated_at)
            VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			request.StockID, stockTxID, request.UserID, "LIMIT", 1, request.Quantity, request.Price, "IN_PROGRESS", now, now,
		).Exec()

	} else {
		err = ordersSession.Query(`
            INSERT INTO orders_keyspace.limit_sell 
            (stock_id, stock_tx_id, user_id, order_type, is_buy, quantity, price, order_status, created_at, updated_at)
            VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			request.StockID, stockTxID, request.UserID, "LIMIT", 0, request.Quantity, request.Price, "IN_PROGRESS", now, now,
		).Exec()
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Data: Error{Message: "Error placing order"}})
		return
	}
	c.JSON(http.StatusOK, Response{Success: true, Data: nil})
}

func cancelLimitOrder(c *gin.Context) {
	userID := checkAuthorization(c)
	if userID == -1 {
		return
	}

	var request CancelRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, Response{Success: false, Data: Error{Message: "Invalid request body"}})
		return
	}

	queries := []string{
		"SELECT stock_tx_id, created_at FROM orders_keyspace.limit_buy WHERE user_id = ? AND stock_id = ?",
		"SELECT stock_tx_id, created_at FROM orders_keyspace.limit_sell WHERE user_id = ? AND stock_id = ?",
	}

	var orderDetails []struct {
		OrderID   string
		CreatedAt time.Time
	}

	for _, query := range queries {
		iter := ordersSession.Query(query, userID, request.StockTxID).Iter()

		var orderID string
		var createdAt time.Time

		for iter.Scan(&orderID, &createdAt) {
			orderDetails = append(orderDetails, struct {
				OrderID   string
				CreatedAt time.Time
			}{OrderID: orderID, CreatedAt: createdAt})
		}
		if err := iter.Close(); err != nil {
			c.JSON(http.StatusInternalServerError, Response{Success: false, Data: Error{Message: "Error fetching order details"}})
			return
		}
	}

	if len(orderDetails) == 0 {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Data: Error{Message: "No orders of given stock_tx_id are found"}})
		return
	}

	for _, order := range orderDetails {
		now := time.Now()
		err := ordersSession.Query(`
			UPDATE orders_keyspace.limit_buy
			SET order_status = 'CANCELLED', updated_at = ?
			WHERE user_id = ? AND stock_id = ? AND stock_tx_id = ? AND created_at = ?`,
			now, userID, request.StockTxID, order.OrderID, order.CreatedAt).Exec()
		if err != nil {
			fmt.Printf("❌ Failed to cancel buy order: %s %v", order.OrderID, err)
			c.JSON(http.StatusInternalServerError, Response{Success: false, Data: Error{Message: "Error cancelling order"}})
			return
		}
		err = ordersSession.Query(`
			UPDATE orders_keyspace.limit_sell
			SET order_status = 'CANCELLED', updated_at = ?
			WHERE user_id = ? AND stock_id = ? AND stock_tx_id = ? AND created_at = ?`,
			now, userID, request.StockTxID, order.OrderID, order.CreatedAt).Exec()
		if err != nil {
			fmt.Printf("❌ Failed to cancel sell order: %s %v", order.OrderID, err)
			c.JSON(http.StatusInternalServerError, Response{Success: false, Data: Error{Message: "Error cancelling order"}})
			return
		}
	}
	c.JSON(http.StatusOK, Response{Success: true, Data: nil})
}

func main() {
	r := gin.Default()

	r.POST("/api/v1/orders/placeStockOrder", placeOrderHandler)
	r.POST("/api/v1/orders/cancelStockTransaction", cancelLimitOrder)
	r.POST("/api/v1/orders/createStock", createStock)
	r.POST("/api/v1/orders/addStockToUser", addStockToUser)

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}

	log.Printf("Server starting on port %s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatal(err)
	}
}
