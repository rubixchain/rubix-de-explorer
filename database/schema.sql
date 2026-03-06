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

CREATE TABLE IF NOT EXISTS "AllTokens" (
    token_id VARCHAR(255) PRIMARY KEY,
    token_type VARCHAR(100)
);

CREATE TABLE IF NOT EXISTS "SC" (
    token_id VARCHAR(255) PRIMARY KEY,
    block_hash VARCHAR(255),
    deployer_did VARCHAR(255),
    executor_did VARCHAR(255),
    block_height BIGINT,
    token_status INTEGER
);

CREATE TABLE IF NOT EXISTS "RBT" (
    token_id VARCHAR(255) PRIMARY KEY,
    owner_did VARCHAR(255),
    block_hash VARCHAR(255),
    block_height VARCHAR(255),
    token_value DOUBLE PRECISION,
    token_status INTEGER
);

CREATE TABLE IF NOT EXISTS "FT" (
    token_id VARCHAR(255) PRIMARY KEY,
    ft_name VARCHAR(255),
    token_value DOUBLE PRECISION,
    owner_did VARCHAR(255),
    creator_did VARCHAR(255),
    block_height BIGINT,
    token_status INTEGER
);

CREATE TABLE IF NOT EXISTS "NFT" (
    token_id VARCHAR(255) PRIMARY KEY,
    token_value TEXT,
    owner_did VARCHAR(255),
    block_height BIGINT,
    token_status INTEGER
);

CREATE TABLE IF NOT EXISTS "DIDs" (
    did VARCHAR(255) PRIMARY KEY,
    total_rbts DOUBLE PRECISION,
    total_fts DOUBLE PRECISION,
    total_nfts BIGINT,
    total_sc BIGINT
);
