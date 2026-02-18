CREATE TABLE IF NOT EXISTS "TransactionBlocks" (
    block_hash TEXT PRIMARY KEY,
    prev_block_id TEXT,
    sender_did TEXT,
    receiver_did TEXT,
    txn_type TEXT,
    amount DOUBLE PRECISION,
    epoch BIGINT,
    tokens JSONB,
    validators JSONB,
    txn_id TEXT
);

CREATE TABLE IF NOT EXISTS "RBT" (
    token_id TEXT PRIMARY KEY,
    owner_did TEXT,
    block_id TEXT,
    block_height TEXT,
    token_value DOUBLE PRECISION,
    token_status INTEGER
);

CREATE TABLE IF NOT EXISTS "FT" (
    token_id TEXT PRIMARY KEY,
    token_value DOUBLE PRECISION,
    ft_name TEXT,
    owner_did TEXT,
    creator_did TEXT,
    block_height BIGINT,
    block_id TEXT,
    txn_id TEXT,
    token_status INTEGER
);

CREATE TABLE IF NOT EXISTS "NFT" (
    token_id TEXT PRIMARY KEY,
    token_value TEXT,
    owner_did TEXT,
    block_hash TEXT,
    txn_id TEXT,
    block_height BIGINT,
    token_status INTEGER
);

CREATE TABLE IF NOT EXISTS "SC" (
    token_id TEXT PRIMARY KEY,
    block_hash TEXT,
    deployer_did TEXT,
    txn_id TEXT,
    block_height BIGINT,
    token_status INTEGER
);

CREATE TABLE IF NOT EXISTS "DIDs" (
    did TEXT PRIMARY KEY,
    created_at TIMESTAMP,
    total_rbts DOUBLE PRECISION,
    total_fts DOUBLE PRECISION,
    total_nfts BIGINT,
    total_sc BIGINT
);

CREATE TABLE IF NOT EXISTS "TxnAnalytics" (
    interval_start TIMESTAMP,
    interval_end TIMESTAMP,
    txn_count BIGINT,
    total_value DOUBLE PRECISION,
    token_type TEXT
);

CREATE TABLE IF NOT EXISTS "AllTokens" (
    token_id VARCHAR(255) PRIMARY KEY,
    token_type VARCHAR(100)
);

CREATE TABLE IF NOT EXISTS "BlockType" (
    block_hash VARCHAR(255) PRIMARY KEY,
    block_type VARCHAR(50),
    epoch TIMESTAMP,
    txn_id VARCHAR(255)
);

CREATE TABLE IF NOT EXISTS "SCBlocks" (
    block_id VARCHAR(255) PRIMARY KEY,
    token_id VARCHAR(255),
    executor_did VARCHAR(255),
    block_height BIGINT,
    epoch TIMESTAMP,
    deployer_did VARCHAR(255)
);

CREATE TABLE IF NOT EXISTS "BurntBlocks" (
    block_hash VARCHAR(255) PRIMARY KEY,
    child_tokens JSONB,
    txn_type VARCHAR(255),
    owner_did VARCHAR(255),
    epoch BIGINT
);

CREATE TABLE IF NOT EXISTS "MintBlocks" (
    block_hash   VARCHAR(255) PRIMARY KEY,
    token_ids    TEXT[] NOT NULL,
    token_type   VARCHAR(50) NOT NULL,
    owner_did    VARCHAR(255) NOT NULL,
    creator_did  VARCHAR(255),
    token_value  DOUBLE PRECISION,
    ft_name      VARCHAR(255),
    epoch        BIGINT,
    txn_type     VARCHAR(50)
);