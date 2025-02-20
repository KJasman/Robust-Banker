-- Enable TimescaleDB extension
CREATE EXTENSION IF NOT EXISTS timescaledb CASCADE;

-- Stock transactions table
CREATE TABLE IF NOT EXISTS stock_transactions (
    stock_tx_id VARCHAR(36),
    parent_stock_tx_id VARCHAR(36),
    stock_id VARCHAR(36) NOT NULL,
    wallet_tx_id VARCHAR(36),
    order_status VARCHAR(20) NOT NULL,
    is_buy BOOLEAN NOT NULL,
    order_type VARCHAR(10) NOT NULL,
    stock_price DECIMAL(18, 2) NOT NULL,
    quantity INTEGER NOT NULL,
    buyer_id VARCHAR(36),
    seller_id VARCHAR(36),
    time_stamp TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (stock_tx_id, time_stamp)
);

-- Create hypertable for time-series optimization
SELECT create_hypertable('stock_transactions', 'time_stamp', if_not_exists => TRUE);

-- Create index on time_stamp for sorting
CREATE INDEX IF NOT EXISTS idx_stock_tx_time ON stock_transactions(time_stamp);
-- Create index on user IDs for faster lookups
CREATE INDEX IF NOT EXISTS idx_stock_tx_buyer ON stock_transactions(buyer_id, time_stamp);
CREATE INDEX IF NOT EXISTS idx_stock_tx_seller ON stock_transactions(seller_id, time_stamp);

-- Wallet transactions table
CREATE TABLE IF NOT EXISTS wallet_transactions (
    wallet_tx_id VARCHAR(36),
    stock_tx_id VARCHAR(36) NOT NULL,
    user_id VARCHAR(36) NOT NULL,
    is_debit BOOLEAN NOT NULL,
    amount DECIMAL(18, 2) NOT NULL,
    time_stamp TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (wallet_tx_id, time_stamp)
);

-- Create hypertable for time-series optimization
SELECT create_hypertable('wallet_transactions', 'time_stamp', if_not_exists => TRUE);

-- Create index on time_stamp for sorting
CREATE INDEX IF NOT EXISTS idx_wallet_tx_time ON wallet_transactions(time_stamp);
-- Create index on user ID for faster lookups
CREATE INDEX IF NOT EXISTS idx_wallet_tx_user ON wallet_transactions(user_id, time_stamp);