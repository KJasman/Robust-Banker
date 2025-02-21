package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/gocql/gocql"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

var (
	cassandraSession  *gocql.Session
	redisClient       *redis.Client
	redisOrderChannel string
)

type apiResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

func main() {
	var err error

	// 1) Connect to Cassandra
	cassandraSession, err = connectCassandra()
	if err != nil {
		log.Fatalf("Failed to connect to Cassandra: %v", err)
	}
	defer cassandraSession.Close()

	// 2) Run migrations
	if err := migrateCassandra(cassandraSession, "./migrations/schema.cql"); err != nil {
		log.Fatalf("Failed to run Cassandra migrations: %v", err)
	}
	log.Println("Cassandra migrations applied successfully.")

	// 3) Connect to Redis
	redisClient, err = connectRedis()
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	defer redisClient.Close()

	// 4) Load environment
	redisOrderChannel = getEnv("REDIS_ORDER_CHANNEL", "new-orders")
	port := getEnv("PORT", "8081")

	// 5) Setup HTTP routes
	r := mux.NewRouter()
	r.HandleFunc("/addStockToUser", addStockToUserHandler).Methods("POST")
	r.HandleFunc("/placeStockOrder", placeStockOrderHandler).Methods("POST")
	r.HandleFunc("/cancelStockTransaction", cancelStockTransactionHandler).Methods("POST")
	r.HandleFunc("/getLowestSellingPrices", getLowestSellingPricesHandler).Methods("POST")

	// Health check
	r.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"order-service OK"}`))
	}).Methods("GET")

	log.Printf("Order-service listening on port %s", port)
	if err := http.ListenAndServe(":"+port, r); err != nil {
		log.Fatalf("Error in ListenAndServe: %v", err)
	}
}

// ----------------------------------------------------------------------------
// CONNECT CASSANDRA
// ----------------------------------------------------------------------------
func connectCassandra() (*gocql.Session, error) {
	cassHost := getEnv("CASSANDRA_DB_HOST", "localhost")
	cassPort := getEnv("CASSANDRA_DB_PORT", "9042")
	ordersKeyspace := getEnv("CASSANDRA_ORDERS_KEYSPACE", "orders_keyspace")

	cluster := gocql.NewCluster(cassHost)
	if p, err := strconv.Atoi(cassPort); err == nil {
		cluster.Port = p
	}
	cluster.Keyspace = ordersKeyspace

	session, err := cluster.CreateSession()
	if err != nil {
		return nil, err
	}

	// Quick test query
	if err := session.Query(`SELECT now() FROM system.local`).Exec(); err != nil {
		return nil, fmt.Errorf("Cassandra ping failed: %w", err)
	}

	log.Printf("Connected to Cassandra (default keyspace: %s, host: %s)", ordersKeyspace, cassHost)
	return session, nil
}

// ----------------------------------------------------------------------------
// MIGRATE CASSANDRA
// ----------------------------------------------------------------------------
func migrateCassandra(session *gocql.Session, cqlFile string) error {
	absPath, err := filepath.Abs(cqlFile)
	if err != nil {
		return fmt.Errorf("unable to get absolute path for migrations file: %v", err)
	}

	f, err := os.Open(absPath)
	if err != nil {
		return fmt.Errorf("failed to open %s: %v", absPath, err)
	}
	defer f.Close()

	content, err := io.ReadAll(f)
	if err != nil {
		return fmt.Errorf("failed to read cql file: %v", err)
	}

	statements := strings.Split(string(content), ";")
	for _, stmt := range statements {
		clean := strings.TrimSpace(stmt)
		if clean == "" {
			continue
		}
		if err := session.Query(clean).Exec(); err != nil {
			return fmt.Errorf("migration error in statement [%s]: %v", clean, err)
		}
	}
	return nil
}

// ----------------------------------------------------------------------------
// CONNECT REDIS
// ----------------------------------------------------------------------------
func connectRedis() (*redis.Client, error) {
	redisHost := getEnv("REDIS_HOST", "localhost")
	redisPort := getEnv("REDIS_PORT", "6379")

	rdb := redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("%s:%s", redisHost, redisPort),
	})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, err
	}
	log.Println("Connected to Redis at", redisHost, redisPort)
	return rdb, nil
}

// ----------------------------------------------------------------------------
// HANDLERS
// ----------------------------------------------------------------------------

// addStockToUser: read-modify-write for a normal int column
func addStockToUserHandler(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserID(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, apiResponse{Success: false, Error: err.Error()})
		return
	}
	log.Printf("[addStockToUser] userID=%d", userID)

	var body struct {
		StockID  int64 `json:"stock_id"`
		Quantity int   `json:"quantity"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, apiResponse{Success: false, Error: "Invalid JSON"})
		return
	}
	if body.StockID <= 0 || body.Quantity <= 0 {
		writeJSON(w, http.StatusBadRequest, apiResponse{Success: false, Error: "stock_id and quantity must be > 0"})
		return
	}

	// 1) Read current quantity from stocks_keyspace.stocks
	var currentQty int
	if err := cassandraSession.Query(`
        SELECT quantity
        FROM stocks_keyspace.stocks
        WHERE stock_id = ?
        LIMIT 1
    `, body.StockID).Scan(&currentQty); err != nil {
		if err == gocql.ErrNotFound {
			currentQty = 0
			// or create row if you'd like
		} else {
			writeJSON(w, http.StatusInternalServerError, apiResponse{Success: false, Error: err.Error()})
			return
		}
	}

	newQty := currentQty + body.Quantity

	// 2) Upsert new quantity
	// If row doesn't exist, create it; else update
	cqlUpsert := `
        INSERT INTO stocks_keyspace.stocks (stock_id, stock_name, quantity, updated_at)
        VALUES (?, 'Unknown', ?, toTimestamp(now()))
    `
	if err := cassandraSession.Query(cqlUpsert, body.StockID, newQty).Exec(); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiResponse{Success: false, Error: err.Error()})
		return
	}

	resp := map[string]interface{}{
		"stock_id": body.StockID,
		"quantity": newQty,
	}
	writeJSON(w, http.StatusOK, apiResponse{Success: true, Data: resp})
}

// placeStockOrder: uses stock_id as int64, no out-of-range errors
func placeStockOrderHandler(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserID(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, apiResponse{Success: false, Error: err.Error()})
		return
	}
	log.Printf("[placeStockOrder] userID=%d", userID)

	var body struct {
		StockID   int64   `json:"stock_id"`
		IsBuy     bool    `json:"is_buy"`
		OrderType string  `json:"order_type"` // "LIMIT" or "MARKET"
		Quantity  int     `json:"quantity"`
		Price     float64 `json:"price"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, apiResponse{Success: false, Error: "Invalid JSON"})
		return
	}
	if body.StockID <= 0 || body.Quantity <= 0 {
		writeJSON(w, http.StatusBadRequest, apiResponse{Success: false, Error: "Invalid stock_id or quantity"})
		return
	}
	if body.OrderType != "MARKET" && body.OrderType != "LIMIT" {
		writeJSON(w, http.StatusBadRequest, apiResponse{Success: false, Error: "order_type must be MARKET or LIMIT"})
		return
	}

	finalPrice := 0.0
	if body.OrderType == "LIMIT" {
		if body.Price <= 0 {
			writeJSON(w, http.StatusBadRequest, apiResponse{Success: false, Error: "Price must be > 0 for LIMIT order"})
			return
		}
		finalPrice = body.Price
	}

	stockTxID := uuid.New()
	createdAt := time.Now()
	orderStatus := "IN_PROGRESS"

	// Insert into orders_by_id
	cql1 := `
    INSERT INTO orders_keyspace.orders_by_id (
        stock_tx_id, parent_stock_tx_id, wallet_tx_id,
        user_id, stock_id, order_type, is_buy, quantity, price, order_status,
        created_at, updated_at
    ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
    `
	if err := cassandraSession.Query(cql1,
		stockTxID.String(), nil, nil,
		int64(userID), // user_id is bigint
		body.StockID,
		body.OrderType, body.IsBuy, body.Quantity, finalPrice, orderStatus,
		createdAt, createdAt,
	).Exec(); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiResponse{Success: false, Error: err.Error()})
		return
	}

	// Insert into orders_by_stock
	cql2 := `
    INSERT INTO orders_keyspace.orders_by_stock (
        stock_id, is_buy, price, stock_tx_id,
        parent_stock_tx_id, wallet_tx_id, user_id, order_type, quantity, order_status,
        created_at, updated_at
    ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
    `
	if err := cassandraSession.Query(cql2,
		body.StockID, body.IsBuy, finalPrice, stockTxID.String(),
		nil, nil, int64(userID), body.OrderType, body.Quantity, orderStatus,
		createdAt, createdAt,
	).Exec(); err != nil {
		// optional rollback from orders_by_id
		writeJSON(w, http.StatusInternalServerError, apiResponse{Success: false, Error: err.Error()})
		return
	}

	// Publish "NEW_ORDER" to Redis
	orderMsg := map[string]interface{}{
		"event":        "NEW_ORDER",
		"stock_tx_id":  stockTxID.String(),
		"stock_id":     body.StockID,
		"is_buy":       body.IsBuy,
		"order_type":   body.OrderType,
		"quantity":     body.Quantity,
		"price":        finalPrice,
		"user_id":      userID,
		"order_status": orderStatus,
		"created_at":   createdAt,
	}
	msgBytes, _ := json.Marshal(orderMsg)
	ctx := context.Background()
	if err := redisClient.Publish(ctx, redisOrderChannel, string(msgBytes)).Err(); err != nil {
		log.Printf("Redis publish error: %v", err)
	}

	resp := map[string]interface{}{
		"stock_tx_id":  stockTxID.String(),
		"order_status": orderStatus,
	}
	writeJSON(w, http.StatusOK, apiResponse{Success: true, Data: resp})
}

// cancelStockTransaction
func cancelStockTransactionHandler(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserID(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, apiResponse{Success: false, Error: err.Error()})
		return
	}
	log.Printf("[cancelStockTransaction] userID=%d", userID)

	var body struct {
		StockTxID string `json:"stock_tx_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, apiResponse{Success: false, Error: "Invalid JSON"})
		return
	}
	if body.StockTxID == "" {
		writeJSON(w, http.StatusBadRequest, apiResponse{Success: false, Error: "Missing stock_tx_id"})
		return
	}

	txUUID, err := uuid.Parse(body.StockTxID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, apiResponse{Success: false, Error: "Invalid UUID"})
		return
	}

	now := time.Now()

	// Mark order as CANCELLED in orders_by_id
	cql1 := `
    UPDATE orders_keyspace.orders_by_id
    SET order_status = ?, updated_at = ?
    WHERE stock_tx_id = ?
    `
	if err := cassandraSession.Query(cql1, "CANCELLED", now, txUUID.String()).Exec(); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiResponse{Success: false, Error: err.Error()})
		return
	}

	// fetch stock_id, isBuy, price from orders_by_id
	var stockID int64
	var isBuy bool
	var price float64
	cql2 := `
    SELECT stock_id, is_buy, price
    FROM orders_keyspace.orders_by_id
    WHERE stock_tx_id = ?
    LIMIT 1
    `
	if err := cassandraSession.Query(cql2, txUUID.String()).Scan(&stockID, &isBuy, &price); err != nil {
		writeJSON(w, http.StatusNotFound, apiResponse{Success: false, Error: "Order not found"})
		return
	}

	//Remove from orders_by_stock
	cql3 := `
    DELETE FROM orders_keyspace.orders_by_stock
    WHERE stock_id = ? AND is_buy = ? AND price = ? AND stock_tx_id = ?
    `
	if err := cassandraSession.Query(cql3, stockID, isBuy, price, txUUID.String()).Exec(); err != nil {
		log.Printf("Warning: failed to remove from orders_by_stock: %v", err)
	}

	// Publish "CANCEL_ORDER" to Redis
	msg := map[string]interface{}{
		"event":       "CANCEL_ORDER",
		"stock_tx_id": txUUID.String(),
		"status":      "CANCELLED",
		"updated_at":  now,
	}
	msgBytes, _ := json.Marshal(msg)
	ctx := context.Background()
	if err := redisClient.Publish(ctx, redisOrderChannel, string(msgBytes)).Err(); err != nil {
		log.Printf("Redis publish error on cancel: %v", err)
	}

	writeJSON(w, http.StatusOK, apiResponse{Success: true, Data: nil})
}

// ----------------------------------------------------------------------------
// HELPERS
// ----------------------------------------------------------------------------
func getUserID(r *http.Request) (int, error) {
	userIDStr := r.Header.Get("X-User-ID")
	if userIDStr == "" {
		return 0, fmt.Errorf("missing X-User-ID in headers")
	}
	uid, err := strconv.Atoi(userIDStr)
	if err != nil {
		return 0, fmt.Errorf("invalid user_id in header")
	}
	return uid, nil
}

func writeJSON(w http.ResponseWriter, code int, resp apiResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(resp)
}

func getEnv(key, defaultVal string) string {
	val := os.Getenv(key)
	if val == "" {
		return defaultVal
	}
	return val
}

func getLowestSellingPricesHandler(w http.ResponseWriter, r *http.Request) {
	var reqBody struct {
		StockIDs []int64 `json:"stock_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		writeJSON(w, http.StatusBadRequest, apiResponse{Success: false, Error: "Invalid JSON"})
		return
	}
	if len(reqBody.StockIDs) == 0 {
		writeJSON(w, http.StatusBadRequest, apiResponse{Success: false, Error: "No stock_ids provided"})
		return
	}

	var responseItems []map[string]interface{}
	for _, stockID := range reqBody.StockIDs {
		currentLowest := findLowestActiveSellPrice(stockID)
		responseItems = append(responseItems, map[string]interface{}{
			"stock_id":             stockID,
			"current_lowest_price": currentLowest,
		})
	}

	writeJSON(w, http.StatusOK, apiResponse{
		Success: true,
		Data:    responseItems,
	})
}

func findLowestActiveSellPrice(stockID int64) float64 {
	cql := `
        SELECT price
        FROM orders_keyspace.orders_by_stock
        WHERE stock_id = ?
          AND is_buy = false
          AND order_status IN ('IN_PROGRESS','PARTIALLY_COMPLETED')
        ALLOW FILTERING
    `
	iter := cassandraSession.Query(cql, stockID).Iter()

	var prices []float64
	var price float64
	for iter.Scan(&price) {
		prices = append(prices, price)
	}
	if err := iter.Close(); err != nil {
		log.Printf("Error in findLowestActiveSellPrice: %v", err)
		return 0
	}

	if len(prices) == 0 {
		return 0
	}
	minPrice := prices[0]
	for _, p := range prices[1:] {
		if p < minPrice {
			minPrice = p
		}
	}
	return minPrice
}
