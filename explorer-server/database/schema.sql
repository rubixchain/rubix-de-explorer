-- =============================================
-- TABLE: TransferBlocks
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
-- TABLE: RBT
-- =============================================
CREATE TABLE IF NOT EXISTS "RBT" (
    rbt_id TEXT PRIMARY KEY,
    owner_did TEXT,
    block_id TEXT,
    block_height TEXT,
    token_value DOUBLE PRECISION,
    token_status INTEGER
);

-- =============================================
-- TABLE: FT
-- =============================================
CREATE TABLE IF NOT EXISTS "FT" (
    ft_id TEXT PRIMARY KEY,
    token_value DOUBLE PRECISION,
    ft_name TEXT,
    owner_did TEXT,
    creator_did TEXT,
    block_height BIGINT,
    block_id TEXT,
    txn_id TEXT,
    token_status INTEGER
);

-- =============================================
-- TABLE: NFT
-- =============================================
CREATE TABLE IF NOT EXISTS "NFT" (
    nft_id TEXT PRIMARY KEY,
    token_value TEXT,
    owner_did TEXT,
    block_hash TEXT,
    txn_id TEXT,
    block_height BIGINT,
    token_status INTEGER
);

-- =============================================
-- TABLE: SmartContract
-- =============================================
CREATE TABLE IF NOT EXISTS "SmartContract" (
    contract_id TEXT PRIMARY KEY,
    block_hash TEXT,
    deployer_did TEXT,
    txn_id TEXT,
    block_height BIGINT,
    token_status INTEGER
);

-- =============================================
-- TABLE: DIDs
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
-- TABLE: TxnAnalytics
-- =============================================
CREATE TABLE IF NOT EXISTS "TxnAnalytics" (
    interval_start TIMESTAMP,
    interval_end TIMESTAMP,
    txn_count BIGINT,
    total_value DOUBLE PRECISION,
    token_type TEXT
);

-- =============================================
-- TABLE: TokenType
-- =============================================
CREATE TABLE IF NOT EXISTS "TokenType" (
    token_id VARCHAR(255) PRIMARY KEY,
    token_type VARCHAR(100),
    last_updated TIMESTAMP NOT NULL
);

-- =============================================
-- TABLE: AllBlocks
-- =============================================
CREATE TABLE IF NOT EXISTS "AllBlocks" (
    block_hash VARCHAR(255) PRIMARY KEY,
    block_type VARCHAR(50),
    epoch TIMESTAMP,
    txn_id VARCHAR(255)
);

-- =============================================
-- TABLE: SC_Blocks
-- =============================================
CREATE TABLE IF NOT EXISTS "SC_Blocks" (
    block_id VARCHAR(255) PRIMARY KEY,
    contract_id VARCHAR(255),
    executor_did VARCHAR(255),
    block_height BIGINT,
    epoch TIMESTAMP,
    owner_did VARCHAR(255)
);

-- =============================================
-- TABLE: BurntBlocks
-- =============================================
CREATE TABLE IF NOT EXISTS "BurntBlocks" (
    block_hash VARCHAR(255) PRIMARY KEY,
    child_tokens JSONB,
    txn_type VARCHAR(255),
    owner_did VARCHAR(255),
    epoch BIGINT,
    tokens JSONB
);
