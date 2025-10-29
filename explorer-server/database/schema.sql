-- =============================================
-- Table: TransferBlocks
-- =============================================
CREATE TABLE IF NOT EXISTS "TransferBlocks" (
    block_hash TEXT PRIMARY KEY,
    prev_block_id TEXT,
    sender_did TEXT,
    receiver_did TEXT,
    txn_type TEXT,
    amount DOUBLE PRECISION,
    epoch BIGINT,
    tokens JSONB,
    validator_pledge_map JSONB,
    txn_id TEXT
);

-- =============================================
-- Table: RBT
-- =============================================
CREATE TABLE IF NOT EXISTS "RBT" (
    token_id TEXT PRIMARY KEY,
    owner_did TEXT,
    block_id TEXT,
    block_height TEXT
);

-- =============================================
-- Table: FT
-- =============================================
CREATE TABLE IF NOT EXISTS "FT" (
    ft_id TEXT PRIMARY KEY,
    token_value DOUBLE PRECISION,
    ft_name TEXT,
    owner_did TEXT,
    creator_did TEXT,
    block_height TEXT,
    block_id TEXT,
    txn_id TEXT
);

-- =============================================
-- Table: NFT
-- =============================================
CREATE TABLE IF NOT EXISTS "NFT" (
    token_id TEXT PRIMARY KEY,
    token_value TEXT,
    owner_did TEXT,
    block_hash TEXT,
    txn_id TEXT
);

-- =============================================
-- Table: SmartContracts
-- =============================================
CREATE TABLE IF NOT EXISTS "SmartContracts" (
    contract_id TEXT PRIMARY KEY,
    block_hash TEXT,
    deployer_did TEXT,
    txn_id TEXT
);

-- =============================================
-- Table: DIDs
-- =============================================
CREATE TABLE IF NOT EXISTS "DIDs" (
    did TEXT PRIMARY KEY,
    created_at TIMESTAMP,
    total_rbts DOUBLE PRECISION,
    total_fts DOUBLE PRECISION,
    total_nfts BIGINT,
    total_sc BIGINT
);

-- =============================================
-- Table: TxnAnalytics
-- =============================================
CREATE TABLE IF NOT EXISTS "TxnAnalytics" (
    interval_start TIMESTAMP,
    interval_end TIMESTAMP,
    txn_count BIGINT,
    total_value DOUBLE PRECISION,
    token_type TEXT
);

-- =============================================
-- Table: TokenTypes
-- =============================================
CREATE TABLE IF NOT EXISTS "TokenTypes" (
    token_id VARCHAR(255) PRIMARY KEY,
    token_type VARCHAR(100),
    last_updated TIMESTAMP NOT NULL DEFAULT NOW()
);

-- =============================================
-- Table: AllBlocks
-- =============================================
CREATE TABLE IF NOT EXISTS "AllBlocks" (
    block_hash VARCHAR(255) PRIMARY KEY,
    block_type VARCHAR(50),
    epoch TIMESTAMP NOT NULL,
    txn_id VARCHAR(255)
);

-- =============================================
-- Table: SC_Blocks
-- =============================================
CREATE TABLE IF NOT EXISTS "SC_Blocks" (
    block_id VARCHAR(255) PRIMARY KEY,
    contract_id VARCHAR(255),
    executor_did VARCHAR(255),
    block_height BIGINT,
    epoch TIMESTAMP NOT NULL,
    owner_did VARCHAR(255)
);

-- =============================================
-- Table: BurntBlocks
-- =============================================
CREATE TABLE IF NOT EXISTS "BurntBlocks" (
    block_hash VARCHAR(255) PRIMARY KEY,
    child_tokens JSONB,
    txn_type VARCHAR(255),
    owner_did VARCHAR(255) NOT NULL,
    epoch BIGINT,
    tokens JSONB
);
