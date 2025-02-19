CREATE TABLE IF NOT EXISTS stocks (
    stock_id int PRIMARY KEY,
    stock_name text,
    quantity int,
    market_price double,
    updated_at timestamp
);
CREATE TABLE IF NOT EXISTS stock_lookup (
    stock_name text PRIMARY KEY,
    stock_id int
);