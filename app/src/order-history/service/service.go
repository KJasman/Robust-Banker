package service

import (
	"context"
	"fmt"
	"time"

	"main/database"
	"main/models"

	"github.com/google/uuid"
)

type TransactionService struct {
	db *database.TimescaleDBHandler
}

func NewTransactionService(db *database.TimescaleDBHandler) *TransactionService {
	return &TransactionService{
		db: db,
	}
}

func (s *TransactionService) RecordStockTransaction(ctx context.Context, tx *models.StockTransaction) error {
	// If there's no time stamp, set it to now
	if tx.TimeStamp.IsZero() {
		tx.TimeStamp = time.Now().UTC()
	}

	// If there's no stock tx ID, generate one
	if tx.StockTxID == "" {
		tx.StockTxID = uuid.New().String()
	}

	query := `
		INSERT INTO stock_transactions (
			stock_tx_id, parent_stock_tx_id, stock_id, wallet_tx_id, 
			order_status, is_buy, order_type, stock_price, 
			quantity, buyer_id, seller_id, time_stamp
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12
		) ON CONFLICT (stock_tx_id) DO UPDATE SET
			parent_stock_tx_id = EXCLUDED.parent_stock_tx_id,
			wallet_tx_id = EXCLUDED.wallet_tx_id,
			order_status = EXCLUDED.order_status,
			is_buy = EXCLUDED.is_buy,
			order_type = EXCLUDED.order_type,
			stock_price = EXCLUDED.stock_price,
			quantity = EXCLUDED.quantity,
			buyer_id = EXCLUDED.buyer_id,
			seller_id = EXCLUDED.seller_id,
			time_stamp = EXCLUDED.time_stamp
	`

	_, err := s.db.GetDB().Exec(ctx, query,
		tx.StockTxID, tx.ParentStockTxID, tx.StockID, tx.WalletTxID,
		tx.OrderStatus, tx.IsBuy, tx.OrderType, tx.StockPrice,
		tx.Quantity, tx.BuyerID, tx.SellerID, tx.TimeStamp,
	)
	if err != nil {
		return fmt.Errorf("failed to record stock transaction: %w", err)
	}

	return nil
}

func (s *TransactionService) RecordWalletTransaction(ctx context.Context, tx *models.WalletTransaction) error {
	// If there's no time stamp, set it to now
	if tx.TimeStamp.IsZero() {
		tx.TimeStamp = time.Now().UTC()
	}

	// If there's no wallet tx ID, generate one
	if tx.WalletTxID == "" {
		tx.WalletTxID = uuid.New().String()
	}

	query := `
		INSERT INTO wallet_transactions (
			wallet_tx_id, stock_tx_id, user_id, 
			is_debit, amount, time_stamp
		) VALUES (
			$1, $2, $3, $4, $5, $6
		) ON CONFLICT (wallet_tx_id) DO UPDATE SET
			stock_tx_id = EXCLUDED.stock_tx_id,
			user_id = EXCLUDED.user_id,
			is_debit = EXCLUDED.is_debit,
			amount = EXCLUDED.amount,
			time_stamp = EXCLUDED.time_stamp
	`

	_, err := s.db.GetDB().Exec(ctx, query,
		tx.WalletTxID, tx.StockTxID, tx.UserID,
		tx.IsDebit, tx.Amount, tx.TimeStamp,
	)
	if err != nil {
		return fmt.Errorf("failed to record wallet transaction: %w", err)
	}

	return nil
}

func (s *TransactionService) GetStockTransactions(ctx context.Context, userID string) ([]models.StockTransaction, error) {
	query := `
		SELECT 
			stock_tx_id, parent_stock_tx_id, stock_id, wallet_tx_id,
			order_status, is_buy, order_type, stock_price,
			quantity, buyer_id, seller_id, time_stamp
		FROM 
			stock_transactions
		WHERE 
			buyer_id = $1 OR seller_id = $1
		ORDER BY 
			time_stamp ASC
	`

	rows, err := s.db.GetDB().Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query stock transactions: %w", err)
	}
	defer rows.Close()

	var transactions []models.StockTransaction
	for rows.Next() {
		var tx models.StockTransaction
		if err := rows.Scan(
			&tx.StockTxID, &tx.ParentStockTxID, &tx.StockID, &tx.WalletTxID,
			&tx.OrderStatus, &tx.IsBuy, &tx.OrderType, &tx.StockPrice,
			&tx.Quantity, &tx.BuyerID, &tx.SellerID, &tx.TimeStamp,
		); err != nil {
			return nil, fmt.Errorf("failed to scan stock transaction: %w", err)
		}
		transactions = append(transactions, tx)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over stock transactions: %w", err)
	}

	return transactions, nil
}

func (s *TransactionService) GetWalletTransactions(ctx context.Context, userID string) ([]models.WalletTransaction, error) {
	query := `
		SELECT 
			wallet_tx_id, stock_tx_id, user_id,
			is_debit, amount, time_stamp
		FROM 
			wallet_transactions
		WHERE 
			user_id = $1
		ORDER BY 
			time_stamp ASC
	`

	rows, err := s.db.GetDB().Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query wallet transactions: %w", err)
	}
	defer rows.Close()

	var transactions []models.WalletTransaction
	for rows.Next() {
		var tx models.WalletTransaction
		if err := rows.Scan(
			&tx.WalletTxID, &tx.StockTxID, &tx.UserID,
			&tx.IsDebit, &tx.Amount, &tx.TimeStamp,
		); err != nil {
			return nil, fmt.Errorf("failed to scan wallet transaction: %w", err)
		}
		transactions = append(transactions, tx)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over wallet transactions: %w", err)
	}

	return transactions, nil
}
