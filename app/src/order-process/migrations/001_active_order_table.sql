-- COCKROACH DB

-- Enable UUID extension (if using UUIDs)
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE buy_orders (
    order_id UUID DEFAULT gen_random_uuid() PRIMARY KEY, -- Prefer UUID for distributed databases
    user_id INT NOT NULL, -- References Authentication Service
    stock_id INT NOT NULL,
    type TEXT NOT NULL CHECK (type IN ('buy')),
    quantity INT NOT NULL,
    price DECIMAL(10,2) NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ DEFAULT now()
    updated_at TIMESTAMPTZ DEFAULT now(),
);

CREATE TABLE sell_orders (
    order_id UUID DEFAULT gen_random_uuid() PRIMARY KEY, 
    user_id INT NOT NULL, -- References Authentication Service
    stock_id INT NOT NULL,
    type TEXT NOT NULL CHECK (type IN ('sell')),
    quantity INT NOT NULL,
    price DECIMAL(10,2) NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now(),
);

