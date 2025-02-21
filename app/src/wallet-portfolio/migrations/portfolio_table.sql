DROP TABLE IF EXISTS stock_portfolio CASCADE;
DROP TABLE IF EXISTS user_wallet CASCADE;
DROP TABLE IF EXISTS stocks CASCADE;

-- 1) Create a sequence for sequential stock IDs
CREATE SEQUENCE IF NOT EXISTS stock_seq 
    START WITH 1 
    INCREMENT BY 1;

-- 2) user_wallet
CREATE TABLE IF NOT EXISTS user_wallet (
    user_id     INT PRIMARY KEY,          -- must match the Auth DB user_id
    balance     DECIMAL(12, 2) NOT NULL DEFAULT 0,
    updated_at  TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp
);

-- 3) stocks table uses 'stock_seq' for stock_id
CREATE TABLE IF NOT EXISTS stocks (
    stock_id     INT PRIMARY KEY DEFAULT nextval('stock_seq'),
    stock_name   TEXT NOT NULL UNIQUE,
    lowest_price DECIMAL(12,2) DEFAULT 0,
    updated_at   TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp
);

-- 4) stock_portfolio
--   user_id + stock_id as composite PK
CREATE TABLE IF NOT EXISTS stock_portfolio (
    user_id        INT NOT NULL, 
    stock_id       INT NOT NULL, 
    quantity_owned INT NOT NULL DEFAULT 0,
    updated_at     TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp,

    PRIMARY KEY (user_id, stock_id),  -- composite key
    FOREIGN KEY (user_id)  REFERENCES user_wallet(user_id) ON DELETE CASCADE,
    FOREIGN KEY (stock_id) REFERENCES stocks(stock_id)     ON DELETE CASCADE
);