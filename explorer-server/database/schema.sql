-- Explorer Database Schema
-- Auto-generated from Go structs

-- 1. Token Types Table
CREATE TABLE IF NOT EXISTS token_types (
    token_id TEXT PRIMARY KEY,
    token_type TEXT,
    last_updated TIMESTAMP NOT NULL
);

-- 2. All Blocks
CREATE TABLE IF NOT EXISTS all_blocks (
    block_hash TEXT PRIMARY KEY,
    block_type BIGINT,
    epoch TIMESTAMP NOT NULL,
    txn_id TEXT
);

-- 3. Transfer Blocks
CREATE TABLE IF NOT EXISTS transfer_blocks (
    block_hash TEXT PRIMARY KEY,
    prev_block_id TEXT,
    sender_did TEXT,
    receiver_did TEXT,
    txn_type TEXT,
    amount DOUBLE PRECISION,
    txn_time TIMESTAMP NOT NULL,
    epoch BIGINT,
    time_taken_ms BIGINT,
    tokens TEXT[],
    validator_pledge_map JSONB,
    txn_id TEXT
);

-- 4. Smart Contracts
CREATE TABLE IF NOT EXISTS smart_contracts (
    contract_id TEXT PRIMARY KEY,
    creator_did TEXT NOT NULL,
    deployer_did TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL,
    last_executed_at TIMESTAMP
);

-- 5. RBT Table
CREATE TABLE IF NOT EXISTS rbts (
    rbt_id TEXT PRIMARY KEY,
    token_value DOUBLE PRECISION NOT NULL,
    owner_did TEXT NOT NULL,
    block_id TEXT NOT NULL,
    block_height TEXT
);

-- 6. FT Table
CREATE TABLE IF NOT EXISTS fts (
    ft_id TEXT PRIMARY KEY,
    token_value DOUBLE PRECISION NOT NULL,
    ft_name TEXT NOT NULL,
    owner_did TEXT NOT NULL,
    creator_did TEXT NOT NULL,
    block_height TEXT,
    block_id TEXT
);

-- 7. NFT Table
CREATE TABLE IF NOT EXISTS nfts (
    nft_id TEXT PRIMARY KEY,
    creator_did TEXT NOT NULL,
    owner_did TEXT NOT NULL,
    block_id TEXT NOT NULL
);

-- 8. DIDs Table
CREATE TABLE IF NOT EXISTS dids (
    did TEXT PRIMARY KEY,
    created_at TIMESTAMP NOT NULL,
    total_rbts DOUBLE PRECISION,
    total_fts DOUBLE PRECISION,
    total_nfts BIGINT,
    total_sc BIGINT
);

-- 9. Transaction Analytics
CREATE TABLE IF NOT EXISTS txn_analytics (
    interval_start TIMESTAMP NOT NULL,
    interval_end TIMESTAMP NOT NULL,
    txn_count BIGINT,
    total_value DOUBLE PRECISION,
    token_type TEXT
);

-- 10. Database Health (for monitoring; usually not persisted)
-- Optional: Can be used for health check logs
CREATE TABLE IF NOT EXISTS database_health (
    is_connected BOOLEAN,
    status TEXT,
    message TEXT
);
