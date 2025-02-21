CREATE EXTENSION IF NOT EXISTS timescaledb CASCADE;

CREATE TABLE IF NOT EXISTS stock_transactions (
    stock_tx_id          TEXT,
    parent_stock_tx_id   TEXT,
    wallet_tx_id         TEXT,
    stock_id             INT NOT NULL,
    user_id              INT NOT NULL,
    order_status         TEXT NOT NULL,
    is_buy               BOOLEAN NOT NULL,
    order_type           TEXT NOT NULL,
    stock_price          DECIMAL(18, 2) NOT NULL,
    quantity             INT NOT NULL,
    time_stamp           TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (stock_tx_id, time_stamp)
);

SELECT create_hypertable('stock_transactions', 'time_stamp', if_not_exists => TRUE);
CREATE INDEX IF NOT EXISTS idx_stx_user ON stock_transactions(user_id, time_stamp);