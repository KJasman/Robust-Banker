-- COCKROACH DB

-- Enable UUID extension (if using UUIDs)
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE market_buy (
    order_id UUID DEFAULT gen_random_uuid() PRIMARY KEY, -- Prefer UUID for distributed databases
    -- user_id INT NOT NULL, -- References Authentication Service
    stock_id INT NOT NULL,
    order_type TEXT NOT NULL DEFAULT 'MARKET',
    is_buy BOOLEAN NOT NULL DEFAULT TRUE,
    quantity INT NOT NULL,
    price DECIMAL(10,2) DEFAULT NULL,
    status TEXT NOT NULL DEFAULT 'IN_PROGRESS',
    created_at TIMESTAMPTZ DEFAULT now()
    updated_at TIMESTAMPTZ DEFAULT now(),
);

CREATE TABLE market_sell (
    order_id UUID DEFAULT gen_random_uuid() PRIMARY KEY, 
    -- user_id INT NOT NULL, -- References Authentication Service
    stock_id INT NOT NULL,
    order_type TEXT NOT NULL DEFAULT 'MARKET',
    is_buy BOOLEAN NOT NULL DEFAULT FALSE,
    quantity INT NOT NULL,
    price DECIMAL(10,2) DEFAULT NULL,
    status TEXT NOT NULL DEFAULT 'IN_PROGRESS',
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now(),
);

CREATE TABLE limit_buy (
    order_id UUID DEFAULT gen_random_uuid() PRIMARY KEY, 
    -- user_id INT NOT NULL, -- References Authentication Service
    stock_id INT NOT NULL,
    order_type TEXT NOT NULL DEFAULT 'LIMIT',
    is_buy BOOLEAN NOT NULL DEFAULT TRUE,
    quantity INT NOT NULL,
    price DECIMAL(10,2) NOT NULL,
    status TEXT NOT NULL DEFAULT 'IN_PROGRESS',
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now(),
);

CREATE TABLE limit_sell (
    order_id UUID DEFAULT gen_random_uuid() PRIMARY KEY, 
    -- user_id INT NOT NULL, -- References Authentication Service
    stock_id INT NOT NULL,
    order_type TEXT NOT NULL DEFAULT 'LIMIT',
    is_buy BOOLEAN NOT NULL DEFAULT FALSE,
    quantity INT NOT NULL,
    price DECIMAL(10,2) NOT NULL,
    status TEXT NOT NULL DEFAULT 'IN_PROGRESS',
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now(),
);

CREATE INDEX idx_market_buy_stock ON market_buy (stock_id);
CREATE INDEX idx_market_sell_stock ON market_sell (stock_id);
CREATE INDEX idx_limit_buy_stock ON limit_buy (stock_id);
CREATE INDEX idx_limit_sell_stock ON limit_sell (stock_id);
