-- TIMESCALE DB

CREATE TABLE buy_orders (
    order_id SERIAL PRIMARY KEY,
    user_id INT NOT NULL, -- References Authentication Service
    stock_id INT NOT NULL,
    type TEXT NOT NULL CHECK (type IN ('buy')),
    quantity INT NOT NULL,
    price DECIMAL(10,2) NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE sell_orders (
    order_id INT PRIMARY KEY,
    user_id INT NOT NULL, -- References Authentication Service
    stock_id INT NOT NULL,
    type TEXT NOT NULL CHECK (type IN ('sell')),
    quantity INT NOT NULL,
    price DECIMAL(10,2) NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    created_at TIMESTAMP DEFAULT NOW()
)

-- Active Order Processing	CockroachDB	Ensures transaction safety & high availability
-- Order History	TimescaleDB	Optimized for querying past orders by time
-- Stock Data	CassandraDB	Scales horizontally, handles fast lookups