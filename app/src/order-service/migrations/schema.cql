-- ============================================
-- CREATE orders_keyspace
-- ============================================
CREATE KEYSPACE IF NOT EXISTS orders_keyspace
  WITH REPLICATION = {
    'class' : 'SimpleStrategy',
    'replication_factor' : 1
  };

-- ============================================
-- orders_keyspace tables
-- ============================================
CREATE TABLE IF NOT EXISTS orders_keyspace.orders_by_id (
    stock_tx_id         uuid PRIMARY KEY,
    parent_stock_tx_id  uuid,
    wallet_tx_id        uuid,
    user_id             bigint,
    stock_id            bigint,
    order_type          text,
    is_buy              boolean,
    quantity            int,
    price               double,
    order_status        text,
    created_at          timestamp,
    updated_at          timestamp
);

CREATE TABLE IF NOT EXISTS orders_keyspace.orders_by_stock (
    stock_id            bigint,
    is_buy              boolean,
    price               double,
    stock_tx_id         uuid,
    parent_stock_tx_id  uuid,
    wallet_tx_id        uuid,
    user_id             bigint,
    order_type          text,
    quantity            int,
    order_status        text,
    created_at          timestamp,
    updated_at          timestamp,
    PRIMARY KEY ((stock_id), is_buy, price, stock_tx_id)
)
WITH CLUSTERING ORDER BY (is_buy ASC, price ASC, stock_tx_id ASC);


-- ============================================
-- CREATE stocks_keyspace
-- ============================================
CREATE KEYSPACE IF NOT EXISTS stocks_keyspace
  WITH REPLICATION = {
    'class' : 'SimpleStrategy',
    'replication_factor' : 1
  };


-- ============================================
-- stocks_keyspace tables
-- ============================================
CREATE TABLE IF NOT EXISTS stocks_keyspace.stocks (
    stock_id   bigint PRIMARY KEY,
    stock_name text,
    quantity   int,
    updated_at timestamp
);

CREATE TABLE IF NOT EXISTS stocks_keyspace.stock_lookup (
    stock_name text PRIMARY KEY,
    stock_id   bigint
);