package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// -----------------------------------------------------------------------------
// NullString for walletTxID
// -----------------------------------------------------------------------------

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

func (ns *NullString) UnmarshalJSON(b []byte) error {
	// If literally "null"
	if string(b) == "null" {
		ns.String = ""
		ns.Valid = false
		return nil
	}
	// Otherwise parse as a normal string
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	ns.String = s
	ns.Valid = true
	return nil
}

// -----------------------------------------------------------------------------
// Stock + Order domain models
// -----------------------------------------------------------------------------

type Stock struct {
	StockID     int       `json:"stock_id"`
	StockName   string    `json:"stock_name"`
	MarketPrice float64   `json:"market_price"`
	Quantity    int       `json:"quantity"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// - StockID: int
// - ParentStockTxID: string (always non-null)
// - WalletTxID: NullString (may be null)
// - Status: string (never null)
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

// -----------------------------------------------------------------------------
// Config + main setup
// -----------------------------------------------------------------------------

type Config struct {
	RedisHost        string
	RedisPort        string
	RedisChannel     string
	WalletServiceURL string
	OrderHistoryURL  string
	Port             string
}

func loadConfig() *Config {
	return &Config{
		RedisHost:        getenv("REDIS_HOST", "redis"),
		RedisPort:        getenv("REDIS_PORT", "6379"),
		RedisChannel:     getenv("REDIS_ORDER_CHANNEL", "new-orders"),
		WalletServiceURL: getenv("WALLET_PORTFOLIO_URL", "http://wallet-service:8083"),
		OrderHistoryURL:  getenv("ORDER_HISTORY_URL", "http://order-history-service:8082"),
		Port:             getenv("PORT", "8084"),
	}
}

func getenv(key, def string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return def
}

var (
	ctx         = context.Background()
	redisClient *redis.Client
	cfg         *Config
)

// -----------------------------------------------------------------------------
// In-memory order book
// -----------------------------------------------------------------------------

type OrderBook struct {
	Buys  []Order
	Sells []Order
	sync.Mutex
}

var (
	books   = make(map[int]*OrderBook)
	booksMu sync.RWMutex
)

func main() {
	cfg = loadConfig()
	if err := initRedis(cfg.RedisHost, cfg.RedisPort); err != nil {
		log.Fatalf("Failed to init Redis: %v", err)
	}
	go subscribeOrders(cfg.RedisChannel)

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"status":"UP","service":"matching-service"}`)
	})

	log.Printf("Matching service on port %s, subscribing to channel=%s", cfg.Port, cfg.RedisChannel)
	if err := http.ListenAndServe(":"+cfg.Port, nil); err != nil {
		log.Fatal(err)
	}
}

// -----------------------------------------------------------------------------
// Redis subscription
// -----------------------------------------------------------------------------

func initRedis(host, port string) error {
	addr := fmt.Sprintf("%s:%s", host, port)
	redisClient = redis.NewClient(&redis.Options{
		Addr: addr,
		DB:   0,
	})
	_, err := redisClient.Ping(ctx).Result()
	return err
}

func subscribeOrders(channel string) {
	sub := redisClient.Subscribe(ctx, channel)
	for {
		msg, err := sub.ReceiveMessage(ctx)
		if err != nil {
			log.Println("Redis subscription error:", err)
			time.Sleep(2 * time.Second)
			continue
		}
		handleOrderEvent(msg.Payload)
	}
}

// -----------------------------------------------------------------------------
// handleOrderEvent
// -----------------------------------------------------------------------------

func handleOrderEvent(payload string) {
	var o Order
	if err := json.Unmarshal([]byte(payload), &o); err != nil {
		log.Println("Failed to unmarshal order JSON:", err)
		return
	}

	// If CANCELLED => remove from in-memory
	if o.Status == "CANCELLED" {
		removeOrder(o.StockTxID)
		return
	}
	// Otherwise => add + attempt matching
	addOrder(o)
	matchOrders(o.StockID)
}

func removeOrder(stockTxID string) {
	booksMu.Lock()
	defer booksMu.Unlock()
	for _, ob := range books {
		ob.Lock()
		ob.Buys = filterOutTxID(ob.Buys, stockTxID)
		ob.Sells = filterOutTxID(ob.Sells, stockTxID)
		ob.Unlock()
	}
}

func filterOutTxID(orders []Order, txID string) []Order {
	out := orders[:0]
	for _, o := range orders {
		if o.StockTxID != txID {
			out = append(out, o)
		}
	}
	return out
}

func addOrder(o Order) {
	booksMu.Lock()
	defer booksMu.Unlock()
	ob, exists := books[o.StockID]
	if !exists {
		ob = &OrderBook{}
		books[o.StockID] = ob
	}
	ob.Lock()
	defer ob.Unlock()

	if o.IsBuy {
		ob.Buys = append(ob.Buys, o)
	} else {
		ob.Sells = append(ob.Sells, o)
	}
}

func matchOrders(stockID int) {
	booksMu.RLock()
	ob := books[stockID]
	booksMu.RUnlock()
	if ob == nil {
		return
	}

	ob.Lock()
	defer ob.Unlock()

	i := 0
	for i < len(ob.Buys) {
		buy := &ob.Buys[i]
		if buy.Quantity <= 0 {
			i++
			continue
		}
		j := 0
		matched := false
		for j < len(ob.Sells) {
			sell := &ob.Sells[j]
			if sell.Quantity <= 0 {
				j++
				continue
			}
			if canMatch(*buy, *sell) {
				qty := min(buy.Quantity, sell.Quantity)
				if err := executeTrade(*buy, *sell, qty); err != nil {
					log.Println("executeTrade error:", err)
				}
				buy.Quantity -= qty
				sell.Quantity -= qty

				if buy.Quantity == 0 {
					buy.Status = "COMPLETED"
					recordFinalTransaction(*buy)
				} else {
					buy.Status = "PARTIALLY_COMPLETE"
				}
				if sell.Quantity == 0 {
					sell.Status = "COMPLETED"
					recordFinalTransaction(*sell)
				} else {
					sell.Status = "PARTIALLY_COMPLETE"
				}
				matched = true
				if buy.Quantity <= 0 {
					break
				}
			}
			j++
		}
		if !matched {
			i++
		} else if buy.Quantity <= 0 {
			i++
		} else {
			i++
		}
	}

	ob.Buys = filterNonZero(ob.Buys)
	ob.Sells = filterNonZero(ob.Sells)
}

func canMatch(buy, sell Order) bool {
	if strings.ToUpper(buy.OrderType) == "MARKET" || strings.ToUpper(sell.OrderType) == "MARKET" {
		return true
	}
	return buy.Price >= sell.Price
}

func filterNonZero(orders []Order) []Order {
	out := orders[:0]
	for _, o := range orders {
		if o.Quantity > 0 {
			out = append(out, o)
		}
	}
	return out
}

// -----------------------------------------------------------------------------
// executeTrade -> uses wallet-service
// -----------------------------------------------------------------------------

func executeTrade(buy, sell Order, qty int) error {
	tradePrice := sell.Price
	if sell.Price == 0 && buy.Price != 0 {
		tradePrice = buy.Price
	}
	cost := float64(qty) * tradePrice

	// 1) Deduct from buyer
	if err := callDeductMoney(buy.UserID, cost); err != nil {
		return fmt.Errorf("deduct buyer error: %w", err)
	}
	// 2) Remove shares from seller
	if err := callUpdatePortfolio(sell.UserID, sell.StockID, -qty); err != nil {
		// rollback buyer money
		_ = callAddMoney(buy.UserID, cost)
		return fmt.Errorf("remove seller shares error: %w", err)
	}
	// 3) Add shares to buyer
	if err := callUpdatePortfolio(buy.UserID, buy.StockID, +qty); err != nil {
		_ = callAddMoney(buy.UserID, cost)
		_ = callUpdatePortfolio(sell.UserID, sell.StockID, qty)
		return fmt.Errorf("add buyer shares error: %w", err)
	}
	// 4) Credit seller
	if err := callAddMoney(sell.UserID, cost); err != nil {
		// revert everything
		_ = callDeductMoney(buy.UserID, cost)
		_ = callUpdatePortfolio(buy.UserID, buy.StockID, -qty)
		_ = callUpdatePortfolio(sell.UserID, sell.StockID, qty)
		return fmt.Errorf("credit seller error: %w", err)
	}
	return nil
}

// -----------------------------------------------------------------------------
// recordFinalTransaction -> calls order-history
// -----------------------------------------------------------------------------

func recordFinalTransaction(o Order) {
	if o.Status != "COMPLETED" {
		return
	}
	body := map[string]interface{}{
		"stock_tx_id":  o.StockTxID,
		"user_id":      o.UserID,
		"stock_id":     o.StockID,
		"quantity":     o.Quantity,
		"price":        o.Price,
		"order_status": "COMPLETED",
	}
	data, _ := json.Marshal(body)

	url := cfg.OrderHistoryURL + "/internal/recordStockTransaction"
	req, _ := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Println("recordFinalTransaction error:", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		rb, _ := io.ReadAll(resp.Body)
		log.Printf("recordFinalTransaction => %d, resp: %s\n", resp.StatusCode, rb)
	}
}

// -----------------------------------------------------------------------------
// wallet-service calls
// -----------------------------------------------------------------------------

func callDeductMoney(userID int, amt float64) error {
	url := cfg.WalletServiceURL + "/deductMoneyFromWallet"
	body := map[string]interface{}{"amount": amt}
	return doWalletCall(url, userID, body)
}

func callAddMoney(userID int, amt float64) error {
	url := cfg.WalletServiceURL + "/addMoneyToWallet"
	body := map[string]interface{}{"amount": amt}
	return doWalletCall(url, userID, body)
}

func callUpdatePortfolio(userID, stockID, delta int) error {
	url := cfg.WalletServiceURL + "/updateStockPortfolio"
	body := map[string]interface{}{"stock_id": stockID, "delta_shares": delta}
	return doWalletCall(url, userID, body)
}

func doWalletCall(url string, userID int, payload interface{}) error {
	b, _ := json.Marshal(payload)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", strconv.Itoa(userID))

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("wallet call failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		rb, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("wallet call got %d: %s", resp.StatusCode, string(rb))
	}
	return nil
}

// utility
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
