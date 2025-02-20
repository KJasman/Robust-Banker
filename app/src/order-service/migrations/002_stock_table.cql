CREATE TABLE IF NOT EXISTS stocks (
    stock_id int,
    stock_name text PRIMARY KEY,
    market_price double,
    updated_at timestamp
);