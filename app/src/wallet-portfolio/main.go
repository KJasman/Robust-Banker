package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	_ "github.com/jackc/pgx/v5/stdlib" // pgx driver
)

// ---------------------------------------------------------------------
// MAIN
// ---------------------------------------------------------------------
func main() {
	// 1) Connect to CockroachDB
	db, err := connectDB()
	if err != nil {
		log.Fatalf("Failed to connect to DB: %v", err)
	}
	defer db.Close()

	// 2) Set up routes
	r := mux.NewRouter()

	// Transaction endpoints
	r.HandleFunc("/addMoneyToWallet", addMoneyToWallet(db)).Methods("POST")
	r.HandleFunc("/getWalletBalance", getWalletBalance(db)).Methods("GET")
	r.HandleFunc("/getStockPortfolio", getStockPortfolio(db)).Methods("GET")
	r.HandleFunc("/getStockPrices", getStockPrices(db)).Methods("GET")

	// Setup endpoint (createStock uses sequence-based stock_id)
	r.HandleFunc("/createStock", createStock(db)).Methods("POST")

	// Health check
	r.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"wallet-portfolio OK"}`))
	}).Methods("GET")

	// 3) Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8083"
	}
	log.Printf("Wallet-Portfolio service is listening on port %s...", port)
	if err := http.ListenAndServe(":"+port, r); err != nil {
		log.Fatalf("ListenAndServe error: %v", err)
	}
}

// ---------------------------------------------------------------------
// DB CONNECTION
// ---------------------------------------------------------------------
func connectDB() (*sql.DB, error) {
	host := getEnvOrDefault("COCKROACH_DB_HOST", "localhost")
	port := getEnvOrDefault("DB_PORT", "26257")
	user := getEnvOrDefault("DB_USER", "root")
	pass := os.Getenv("DB_PASSWORD")
	dbName := getEnvOrDefault("DB_NAME", "defaultdb")

	// Example DSN => "postgresql://user:pass@host:26257/dbname?sslmode=disable"
	dsn := fmt.Sprintf("postgresql://%s:%s@%s:%s/%s?sslmode=disable", user, pass, host, port, dbName)
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}

	// Ping with some retry
	if err := pingWithRetry(db, 5); err != nil {
		return nil, fmt.Errorf("unable to connect after retries: %w", err)
	}
	log.Println("Connected to CockroachDB successfully.")

	return db, nil
}

func pingWithRetry(db *sql.DB, attempts int) error {
	var err error
	for i := 0; i < attempts; i++ {
		err = db.Ping()
		if err == nil {
			return nil
		}
		log.Printf("DB ping failed (attempt %d/%d): %v", i+1, attempts, err)
		time.Sleep(2 * time.Second)
	}
	return err
}

func getEnvOrDefault(key, defVal string) string {
	val := os.Getenv(key)
	if val == "" {
		return defVal
	}
	return val
}

// ---------------------------------------------------------------------
// HANDLERS (ENDPOINTS)
// ---------------------------------------------------------------------

type apiResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// 1) POST /addMoneyToWallet
// Body: { "amount": 10000 }
func addMoneyToWallet(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, apiResponse{Success: false, Error: "Method not allowed"})
			return
		}

		userID, err := getUserID(r)
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, apiResponse{Success: false, Error: err.Error()})
			return
		}

		var body struct {
			Amount float64 `json:"amount"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, apiResponse{Success: false, Error: "Invalid JSON"})
			return
		}
		if body.Amount <= 0 {
			writeJSON(w, http.StatusBadRequest, apiResponse{Success: false, Error: "Amount must be positive"})
			return
		}

		tx, err := db.Begin()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, apiResponse{Success: false, Error: err.Error()})
			return
		}
		defer tx.Rollback()

		_, err = tx.Exec(`
            INSERT INTO user_wallet (user_id, balance)
            VALUES ($1, $2)
            ON CONFLICT (user_id) DO UPDATE
              SET balance = user_wallet.balance + EXCLUDED.balance,
                  updated_at = NOW()
        `, userID, body.Amount)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, apiResponse{Success: false, Error: err.Error()})
			return
		}

		if err := tx.Commit(); err != nil {
			writeJSON(w, http.StatusInternalServerError, apiResponse{Success: false, Error: err.Error()})
			return
		}

		writeJSON(w, http.StatusOK, apiResponse{Success: true, Data: nil})
	}
}

// 2) POST /createStock
// Insert a row into `stocks` with `DEFAULT nextval('stock_seq')` for stock_id
func createStock(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, apiResponse{Success: false, Error: "Method not allowed"})
			return
		}

		var body struct {
			StockName string `json:"stock_name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, apiResponse{Success: false, Error: "Invalid JSON"})
			return
		}
		if body.StockName == "" {
			writeJSON(w, http.StatusBadRequest, apiResponse{Success: false, Error: "Missing stock_name"})
			return
		}

		var newStockID int
		err := db.QueryRow(`
            INSERT INTO stocks (stock_name)
            VALUES ($1)
            RETURNING stock_id
        `, body.StockName).Scan(&newStockID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, apiResponse{Success: false, Error: err.Error()})
			return
		}

		resp := map[string]interface{}{
			"stock_id": newStockID,
		}
		writeJSON(w, http.StatusOK, apiResponse{Success: true, Data: resp})
	}
}

// 3) GET /getStockPortfolio
func getStockPortfolio(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, apiResponse{Success: false, Error: "Method not allowed"})
			return
		}

		userID, err := getUserID(r)
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, apiResponse{Success: false, Error: err.Error()})
			return
		}

		rows, err := db.Query(`
            SELECT sp.stock_id, s.stock_name, sp.quantity_owned
            FROM stock_portfolio sp
            JOIN stocks s ON sp.stock_id = s.stock_id
            WHERE sp.user_id = $1
            ORDER BY s.stock_name
        `, userID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, apiResponse{Success: false, Error: err.Error()})
			return
		}
		defer rows.Close()

		var portfolio []map[string]interface{}
		for rows.Next() {
			var stockID, quantity int
			var stockName string
			if err := rows.Scan(&stockID, &stockName, &quantity); err != nil {
				writeJSON(w, http.StatusInternalServerError, apiResponse{Success: false, Error: err.Error()})
				return
			}
			item := map[string]interface{}{
				"stock_id":       stockID,
				"stock_name":     stockName,
				"quantity_owned": quantity,
			}
			portfolio = append(portfolio, item)
		}

		writeJSON(w, http.StatusOK, apiResponse{Success: true, Data: portfolio})
	}
}

// 4) GET /getWalletBalance
func getWalletBalance(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, apiResponse{Success: false, Error: "Method not allowed"})
			return
		}

		userID, err := getUserID(r)
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, apiResponse{Success: false, Error: err.Error()})
			return
		}

		var balance float64
		err = db.QueryRow(`SELECT balance FROM user_wallet WHERE user_id = $1`, userID).Scan(&balance)
		if err == sql.ErrNoRows {
			writeJSON(w, http.StatusNotFound, apiResponse{Success: false, Error: "No wallet for this user"})
			return
		} else if err != nil {
			writeJSON(w, http.StatusInternalServerError, apiResponse{Success: false, Error: err.Error()})
			return
		}

		writeJSON(w, http.StatusOK, apiResponse{
			Success: true,
			Data: map[string]interface{}{
				"balance": balance,
			},
		})
	}
}

// 5) GET /getStockPrices
func getStockPrices(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, apiResponse{Success: false, Error: "Method not allowed"})
			return
		}

		userID, err := getUserID(r)
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, apiResponse{Success: false, Error: err.Error()})
			return
		}

		rows, err := db.Query(`
            SELECT sp.stock_id, s.stock_name
            FROM stock_portfolio sp
            JOIN stocks s ON sp.stock_id = s.stock_id
            WHERE sp.user_id = $1
            ORDER BY s.stock_name
        `, userID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, apiResponse{Success: false, Error: err.Error()})
			return
		}
		defer rows.Close()

		var stockIDs []int
		var portfolioData []map[string]interface{}
		for rows.Next() {
			var stockID int
			var stockName string
			if err := rows.Scan(&stockID, &stockName); err != nil {
				writeJSON(w, http.StatusInternalServerError, apiResponse{Success: false, Error: err.Error()})
				return
			}
			portfolioData = append(portfolioData, map[string]interface{}{
				"stock_id":   stockID,
				"stock_name": stockName,
			})
			stockIDs = append(stockIDs, stockID)
		}

		if len(stockIDs) == 0 {
			writeJSON(w, http.StatusOK, apiResponse{Success: true, Data: []interface{}{}})
			return
		}

		lowestPrices, err := fetchLowestSellingPricesFromOrderService(stockIDs)
		if err != nil {
			log.Printf("Error fetching lowest prices: %v", err)
			writeJSON(w, http.StatusBadGateway, apiResponse{Success: false, Error: "Could not retrieve lowest prices"})
			return
		}

		priceMap := make(map[int]float64)
		for _, item := range lowestPrices {
			sid := int(item["stock_id"].(float64))
			price := item["current_lowest_price"].(float64)
			priceMap[sid] = price
		}

		var result []map[string]interface{}
		for _, p := range portfolioData {
			sid := p["stock_id"].(int)
			lowestP, ok := priceMap[sid]
			if !ok {
				lowestP = 0
			}
			result = append(result, map[string]interface{}{
				"stock_id":             sid,
				"stock_name":           p["stock_name"],
				"current_lowest_price": lowestP,
			})
		}

		writeJSON(w, http.StatusOK, apiResponse{Success: true, Data: result})
	}
}

// ---------------------------------------------------------------------
// HELPER FUNCTIONS
// ---------------------------------------------------------------------

func getUserID(r *http.Request) (int, error) {
	uidStr := r.Header.Get("X-User-ID")
	if uidStr == "" {
		return 0, fmt.Errorf("missing X-User-ID header")
	}
	uid, err := strconv.Atoi(uidStr)
	if err != nil {
		return 0, fmt.Errorf("invalid user ID")
	}
	return uid, nil
}

func fetchLowestSellingPricesFromOrderService(stockIDs []int) ([]map[string]interface{}, error) {
	type requestBody struct {
		StockIDs []int `json:"stock_ids"`
	}
	reqPayload := requestBody{StockIDs: stockIDs}
	jsonBody, _ := json.Marshal(reqPayload)

	url := "http://order-service:8081/engine/getLowestSellingPrices"
	req, err := http.NewRequest("POST", url, strings.NewReader(string(jsonBody)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("order-service returned %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var respData struct {
		Success bool          `json:"success"`
		Data    []interface{} `json:"data"`
		Error   string        `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&respData); err != nil {
		return nil, err
	}
	if !respData.Success {
		return nil, fmt.Errorf("order-service error: %s", respData.Error)
	}

	var result []map[string]interface{}
	for _, raw := range respData.Data {
		if m, ok := raw.(map[string]interface{}); ok {
			result = append(result, m)
		}
	}
	return result, nil
}

func writeJSON(w http.ResponseWriter, code int, resp apiResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(resp)
}
