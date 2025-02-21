DROP TABLE IF EXISTS wallet_transactions;
DROP TABLE IF EXISTS stock_portfolio;
DROP TABLE IF EXISTS wallet;

CREATE TABLE IF NOT EXISTS wallet (
    wallet_id VARCHAR(36) PRIMARY KEY,
    user_id SERIAL UNIQUE NOT NULL,
    balance DECIMAL(10, 2) NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS stock_portfolio (
    portfolio_id SERIAL PRIMARY KEY,
    wallet_id VARCHAR(36) NOT NULL,
    stock_id SERIAL NOT NULL,
    quantity_owned INTEGER NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (wallet_id) REFERENCES wallet(wallet_id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS wallet_transactions (
    wallet_tx_id VARCHAR(36) PRIMARY KEY,
    wallet_id VARCHAR(36) NOT NULL,
    stock_tx_id VARCHAR(100),
    is_debit BOOLEAN NOT NULL,
    amount DECIMAL(20, 2) NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (wallet_id) REFERENCES wallet(wallet_id) ON DELETE CASCADE
);