CREATE TABLE IF NOT EXISTS stocks (
    stock_id UUID,
    stock_name text PRIMARY KEY,
    market_price double,
    updated_at timestamp
);