package models

import (
	"time"
)

type StockTransaction struct {
	StockTxID       string    `json:"stock_tx_id"`
	ParentStockTxID *string   `json:"parent_stock_tx_id"`
	StockID         string    `json:"stock_id"`
	WalletTxID      *string   `json:"wallet_tx_id"`
	OrderStatus     string    `json:"order_status"`
	IsBuy           bool      `json:"is_buy"`
	OrderType       string    `json:"order_type"`
	StockPrice      float64   `json:"stock_price"`
	Quantity        int       `json:"quantity"`
	BuyerID         *string   `json:"buyer_id,omitempty"`
	SellerID        *string   `json:"seller_id,omitempty"`
	TimeStamp       time.Time `json:"time_stamp"`
}

type WalletTransaction struct {
	WalletTxID string    `json:"wallet_tx_id"`
	StockTxID  string    `json:"stock_tx_id"`
	UserID     string    `json:"user_id"`
	IsDebit    bool      `json:"is_debit"`
	Amount     float64   `json:"amount"`
	TimeStamp  time.Time `json:"time_stamp"`
}
