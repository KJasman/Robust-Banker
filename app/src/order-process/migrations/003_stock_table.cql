# CASSANDRA DB
CREATE TABLE IF NOT EXISTS stocks (
    stock_id int PRIMARY KEY,
    name text,
    market_price float,
    quantity int,
    updated_at timestamp
);

