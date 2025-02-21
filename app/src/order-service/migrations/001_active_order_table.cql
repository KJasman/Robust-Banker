-- Keyspace: orders_keyspace

CREATE TABLE IF NOT EXISTS orders_keyspace.market_buy (
    stock_id int,
    stock_tx_id uuid,
    parent_stock_tx_id uuid,
    wallet_tx_id uuid,
    user_id int, 
    order_type text,
    is_buy boolean,
    quantity int,
    price double, 
    order_status text,
    created_at timestamp,
    updated_at timestamp,
    PRIMARY KEY ((user_id, stock_id), created_at, stock_tx_id)
);

CREATE TABLE IF NOT EXISTS orders_keyspace.market_sell (
    stock_id int,
    stock_tx_id uuid,
    parent_stock_tx_id uuid,
    wallet_tx_id uuid,
    user_id int,
    order_type text,
    is_buy boolean,
    quantity int,
    price double, 
    order_status text,
    created_at timestamp,
    updated_at timestamp,
    PRIMARY KEY ((user_id, stock_id), created_at, stock_tx_id)
);

CREATE TABLE IF NOT EXISTS orders_keyspace.limit_buy (
    stock_id int,
    stock_tx_id uuid,
    parent_stock_tx_id uuid,
    wallet_tx_id uuid,
    user_id int,
    order_type text,
    is_buy boolean,
    quantity int,
    price double,
    order_status text,
    created_at timestamp,
    updated_at timestamp,
    PRIMARY KEY ((user_id, stock_id), created_at, stock_tx_id)
);

CREATE TABLE IF NOT EXISTS orders_keyspace.limit_sell (
    stock_id int,
    stock_tx_id uuid,
    parent_stock_tx_id uuid,
    wallet_tx_id uuid,
    user_id int,
    order_type text,
    is_buy boolean,
    quantity int,
    price double,
    order_status text,
    created_at timestamp,
    updated_at timestamp,
    PRIMARY KEY ((user_id, stock_id), created_at, stock_tx_id)
);