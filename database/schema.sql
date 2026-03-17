CREATE TABLE IF NOT EXISTS "AllBlocks" (
    block_hash VARCHAR(255) PRIMARY KEY,
    txn_id VARCHAR(255),
    block_type VARCHAR(50)
);

CREATE TABLE IF NOT EXISTS "TransactionBlocks" (
    block_hash TEXT PRIMARY KEY,
    txn_id TEXT,
    sender_did TEXT,
    receiver_did TEXT,
    asset_type TEXT,
    amount DOUBLE PRECISION,
    token_count INTEGER,
    epoch BIGINT,
    tokens JSONB,
    validators JSONB
);

CREATE TABLE IF NOT EXISTS "SCBlocks" (
    block_hash VARCHAR(255) PRIMARY KEY,
    token_id VARCHAR(255),
    executor_did VARCHAR(255),
    deployer_did VARCHAR(255),
    block_height BIGINT,
    epoch TIMESTAMP
);

CREATE TABLE IF NOT EXISTS "BurntBlocks" (
    block_hash VARCHAR(255) PRIMARY KEY,
    tokens JSONB,
    owner_did VARCHAR(255),
    epoch BIGINT
);

CREATE TABLE IF NOT EXISTS "MintBlocks" (
    block_hash VARCHAR(255) PRIMARY KEY,
    token_ids TEXT[] NOT NULL,
    token_type VARCHAR(50) NOT NULL,
    token_value DOUBLE PRECISION,
    creator_did VARCHAR(255) NOT NULL,
    ft_name VARCHAR(255),
    epoch BIGINT
);

CREATE TABLE IF NOT EXISTS "Tokens" (
    token_id VARCHAR(255) PRIMARY KEY,
    token_type VARCHAR(50) NOT NULL, -- 'RBT', 'FT', 'NFT', 'SC'
    token_value DOUBLE PRECISION DEFAULT 0,
    owner_did VARCHAR(255),
    creator_did VARCHAR(255),
    ft_name VARCHAR(255),
    block_hash VARCHAR(255),
    block_height BIGINT,
    token_status INTEGER DEFAULT 1 -- 1: Active, 0: Burnt/Inactive
);

CREATE INDEX IF NOT EXISTS idx_tokens_owner ON "Tokens"(owner_did);
CREATE INDEX IF NOT EXISTS idx_tokens_type ON "Tokens"(token_type);

CREATE TABLE IF NOT EXISTS "TokenChain" (
    id SERIAL PRIMARY KEY,
    token_id VARCHAR(255) NOT NULL,
    transaction_id VARCHAR(255) NOT NULL,
    previous_transaction_id VARCHAR(255),
    role VARCHAR(50)
);

CREATE INDEX IF NOT EXISTS idx_tokenchain_id ON "TokenChain"(token_id);

CREATE TABLE IF NOT EXISTS "TokenChainIndex" (
    token_id VARCHAR(255) PRIMARY KEY,
    index JSONB -- Array of transaction IDs Index from TokenChain table
);

CREATE TABLE IF NOT EXISTS "DIDBalances" (
    did VARCHAR(255) NOT NULL,
    asset_type VARCHAR(50) NOT NULL,
    token_name VARCHAR(255) NOT NULL, -- 'RBT' or 'APPLE', etc.
    balance DOUBLE PRECISION DEFAULT 0,
    last_update BIGINT,
    PRIMARY KEY (did, asset_type, token_name)
);

CREATE INDEX IF NOT EXISTS idx_didbalances_asset ON "DIDBalances"(asset_type, token_name);
CREATE INDEX IF NOT EXISTS idx_didbalances_balance ON "DIDBalances"(balance DESC);
