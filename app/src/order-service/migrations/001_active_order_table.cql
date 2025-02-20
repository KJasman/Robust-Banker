CREATE TABLE IF NOT EXISTS market_buy (
    stock_id int,
    stock_tx_id UUID,
    parent_stock_tx_id UUID,
    wallet_tx_id UUID,
    user_id int, 
    order_type text,
    is_buy int,
    quantity int,
    price double, 
    order_status text,
    created_at timestamp,
    updated_at timestamp,
    PRIMARY KEY ((user_id, stock_id), created_at, stock_tx_id)
);
CREATE TABLE IF NOT EXISTS market_sell (
    stock_id int,
    stock_tx_id UUID,
    parent_stock_tx_id UUID,
    wallet_tx_id UUID,
    user_id int,
    order_type text,
    is_buy int,
    quantity int,
    price double, 
    order_status text,
    created_at timestamp,
    updated_at timestamp,
    PRIMARY KEY ((user_id, stock_id), created_at, stock_tx_id)
);
CREATE TABLE IF NOT EXISTS limit_buy (
    stock_id int,
    stock_tx_id UUID,
    parent_stock_tx_id UUID,
    wallet_tx_id UUID,
    user_id int,
    order_type text,
    is_buy int,
    quantity int,
    price double, 
    order_status text,
    created_at timestamp,
    updated_at timestamp,
    PRIMARY KEY ((user_id, stock_id), created_at, stock_tx_id)
);
CREATE TABLE IF NOT EXISTS limit_sell (
    stock_id int,
    stock_tx_id UUID,
    parent_stock_tx_id UUID,
    wallet_tx_id UUID,
    user_id int,
    order_type text,
    is_buy int,
    quantity int,
    price double, 
    order_status text,
    created_at timestamp,
    updated_at timestamp,
    PRIMARY KEY ((user_id, stock_id), created_at, stock_tx_id)
);