-- Keyspace: stocks_keyspace

CREATE TABLE IF NOT EXISTS stocks_keyspace.stocks (
    stock_id int PRIMARY KEY,
    stock_name text,
    quantity int,
    market_price double,
    updated_at timestamp
);

CREATE TABLE IF NOT EXISTS stocks_keyspace.stock_lookup (
    stock_name text PRIMARY KEY,
    stock_id int
);