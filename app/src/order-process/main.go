package main

import (
	"database/sql"
	"encoding/json"
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
	UserID int `json:"user_id"` // Need later for > 1 user
	// OrderID   int       `json:"order_id"`
	StockID   int       `json:"stock_id"`
	StockData Stock     `json:"stock_data"`
	OrderType string    `json:"order_type"`
	IsBuy     string    `json:"is_buy"`
	Quantity  int       `json:"quantity"`
	Price     float64   `json:"price"`
	Status    string    `json:"order_status"`
	Created   time.Time `json:"created"`
}

type Stock struct {
	StockID     int       `json:"stock_id"`
	Name        string    `json:"name"`
	MarketPrice float64   `json:"market_price"`
	Quantity    int       `json:"quantity"`
	Updated     time.Time `json:"updated"`
}

type User struct {
	UserID  int     `json:"user_id"`
	Balance float64 `json:"balance"`
}

type Wallet struct {
	UserID  int     `json:"user_id"`
	Balance float64 `json:"balance"`
}

type Response struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data"`
}

var (
	timescaleDB *sql.DB
	cockroachDB *sql.DB
	cassandraDB *gocql.Session
)

func buildDatabaseURL(hostEnv string, portEnv string, nameEnv string) string {
	host := os.Getenv(hostEnv)
	port := os.Getenv(portEnv)
	user := os.Getenv("DB_USER")
	password := os.Getenv("DB_PASSWORD")
	dbname := os.Getenv(nameEnv)
	sslmode := os.Getenv("DB_SSLMODE")

	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		host, port, user, password, dbname, sslmode,
	)
}

func initDB() error {
	// Initialize database connection
	var err error
	ts := buildDatabaseURL("TIMESCALE_DB_HOST", "TIMESCALE_DB_PORT", "TIMESCALE_DB_NAME")
	timescaleDB, err := sql.Open("postgres", ts)
	if err != nil {
		return fmt.Errorf("failed to open database: %v", err)
	}
	if err = timescaleDB.Ping(); err != nil {
		timescaleDB.Close()
		return fmt.Errorf("❌error connecting to the database: %v", err)
	}
	fmt.Println("✅Connected to TimescaleDB successfully!")

	cr := buildDatabaseURL("COCKROACH_DB_HOST", "COCKROACH_DB_PORT", "COCKROACH_DB_NAME")
	cockroachDB, err := sql.Open("postgres", cr)
	if err != nil {
		return fmt.Errorf("failed to open database: %v", err)
	}
	if err = cockroachDB.Ping(); err != nil {
		cockroachDB.Close()
		return fmt.Errorf("❌error connecting to the database: %v", err)
	}
	fmt.Println("✅Connected to CockroachDB successfully!")

	cluster := gocql.NewCluster(os.Getenv("CASSANDRA_DB_HOST"))
	cluster.Port, _ = strconv.Atoi(os.Getenv("CASSANDRA_DB_PORT"))
	cluster.Keyspace = os.Getenv("CASSANDRA_DB_KEYSPACE")
	cluster.Authenticator = gocql.PasswordAuthenticator{
		Username: os.Getenv("DB_USER"),
		Password: os.Getenv("DB_PASSWORD"),
	}
	cluster.Consistency = gocql.Quorum

	cassandraDB, err = cluster.CreateSession()
	if err != nil {
		return fmt.Errorf("❌error connecting to Cassandra: %v", err)
	}
	fmt.Println("✅Connected to CassandraDB successfully!")

	return applyMigrations()
}

func applyMigrations() error {
	// CochroachDB & TimescaleDB
	migrations := map[*sql.DB]string{
		cockroachDB: "migrations/001_active_order_table.sql",
		timescaleDB: "migrations/002_history_order_table.sql",
	}
	for db, filePath := range migrations {
		migration, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("error reading migration file %s: %v", filePath, err)
		}

		_, err = db.Exec(string(migration))
		if err != nil {
			return fmt.Errorf("❌error applying migration %s: %v", filePath, err)
		}

		log.Printf("✅Migration %s applied successfully\n", filePath)
	}

	// CassandraDB
	csd := "migrations/003_stock_table.cql"
	migration, err := os.ReadFile(csd)
	if err != nil {
		return fmt.Errorf("error reading migration file %s: %v", csd, err)
	}

	migrationQueries := strings.Split(string(migration), ";")
	for _, query := range migrationQueries {
		if query != "" {
			err := cassandraDB.Query(query).Exec()
			if err != nil {
				return fmt.Errorf("❌error applying migration %s: %v", csd, err)
			}
		}
	}
	log.Printf("✅Migration %s applied successfully\n", csd)

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

func getUserBalance(userID int) float64 {
	walletServiceURL := fmt.Sprintf("http://localhost:8000/api/v1/wallet/balance?user_id=%d", userID)

	connected, err := http.Get(walletServiceURL)
	if err != nil {
		log.Println("Error connecting Wallet Service: ", err)
		return 0
	}
	defer connected.Body.Close()

	var response Response

	if err := json.NewDecoder(connected.Body).Decode(&response); err != nil {
		log.Println("Error decoding response:", err)
		return 0
	}
	var balance float64
	if response.Success {
		balance = response.Data.Wallet.Balance
	}
	return balance
}

func placeOrderHandler(c *gin.Context) {
	// Parse request body
	var request Order
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Validate request
	if request.Quantity <= 0 || request.Price <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid quantity or price"})
		return
	}

	balance := getUserBalance(request.UserID)

	if request.Price > balance {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Insufficient balance"})
		return
	}
	if request.OrderType == "MARKET" {
		placeMarketOrder(request, c)
	} else if request.OrderType == "LIMIT" {
		placeLimitOrder(request, c)
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid order type"})
	}
}

func placeMarketOrder(request Order, c *gin.Context) {
	// Market order, buy/sell immediate stock at current price 
	var orderID int
	var err error

	if request.IsBuy == true {
		err = cockroachDB.QueryRow( // does the uninitialized value will be set with default value?
			"INSERT INTO market_buy (stock_id, order_type, quantity, price, created) VALUES ($1, $2, $3, $4, $5) RETURNING order_id",
			request.StockID, request.OrderType, request.Quantity, request.Price, time.Now(),
		).Scan(&orderID)

	} else if request.IsBuy == false {
		err = cockroachDB.QueryRow(// is it nescessary to take price input as the default value is null?
			"INSERT INTO market_sell (stock_id, order_type, quantity, price, created) VALUES ($1, $2, $3, $4, $5) RETURNING order_id",
			request.StockID, request.OrderType, request.Quantity, request.Price, time.Now(),
		).Scan(&orderID) // what does this do? do i need to rearrange the order by any specific order for the matching engine?
	}
	// should i have active and completed order proess in the same server?

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create order"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Order created successfully", "order_id": orderID})
}

// func placeLimitOrder(request Order, c *gin.Context) {
// 	// Sort buy order in descending by bid price and ascending time for those with same price
// 	// sell oder in ascending by ask price and ascending time for same price
// }

// funv cancelLimitOrder() {

// }

func main() {
	r := gin.Default()

	r.POST("engine/placeStockOrder", placeOrderHandler)

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
