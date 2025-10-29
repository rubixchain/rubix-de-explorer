-- Table: transfer_blocks
CREATE TABLE IF NOT EXISTS transfer_blocks (
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

-- Table: rbt
CREATE TABLE IF NOT EXISTS rbt (
    token_id TEXT PRIMARY KEY,
    owner_did TEXT,
    block_id TEXT,
    block_height TEXT
);

-- Table: ft
CREATE TABLE IF NOT EXISTS ft (
    ft_id TEXT PRIMARY KEY,
    token_value DOUBLE PRECISION,
    ft_name TEXT,
    owner_did TEXT,
    creator_did TEXT,
    block_height TEXT,
    block_id TEXT,
    txn_id TEXT
);

-- Table: nft
CREATE TABLE IF NOT EXISTS nft (
    token_id TEXT PRIMARY KEY,
    token_value TEXT,
    owner_did TEXT,
    block_hash TEXT,
    txn_id TEXT
);

-- Table: smart_contracts
CREATE TABLE IF NOT EXISTS smart_contracts (
    contract_id TEXT PRIMARY KEY,
    block_hash TEXT,
    deployer_did TEXT,
    txn_id TEXT
);

-- Table: dids
CREATE TABLE IF NOT EXISTS dids (
    did TEXT PRIMARY KEY,
    created_at TIMESTAMP,
    total_rbts DOUBLE PRECISION,
    total_fts DOUBLE PRECISION,
    total_nfts BIGINT,
    total_sc BIGINT
);

-- Table: txn_analytics
CREATE TABLE IF NOT EXISTS txn_analytics (
    interval_start TIMESTAMP,
    interval_end TIMESTAMP,
    txn_count BIGINT,
    total_value DOUBLE PRECISION,
    token_type TEXT
);

CREATE TABLE IF NOT EXISTS token_types (
    token_id VARCHAR(255) PRIMARY KEY,
    token_type VARCHAR(100),
    last_updated TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS all_blocks (
    block_hash VARCHAR(255) PRIMARY KEY,
    block_type BIGINT,
    epoch TIMESTAMP NOT NULL,
    txn_id VARCHAR(255)
);

CREATE TABLE IF NOT EXISTS sc_blocks (
    contract_id VARCHAR(255) PRIMARY KEY,
    executor_did VARCHAR(255),
    block_height BIGINT,
    epoch TIMESTAMP NOT NULL,
    owner_did VARCHAR(255)
);


CREATE TABLE IF NOT EXISTS burntblocks (
    block_hash VARCHAR(255) PRIMARY KEY,
    child_tokens JSONB,                   
    txn_type VARCHAR(255),                
    owner_did VARCHAR(255) NOT NULL,      
    epoch BIGINT,                         
    tokens JSONB                          
);