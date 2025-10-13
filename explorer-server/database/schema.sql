-- Explorer Database Schema for Decentralized Token System
-- This file contains the schema for the decentralized explorer database

-- Use the database
-- \c explorer_db;

-- 1. Tokens Table
-- Stores all tokens and their metadata.
CREATE TABLE IF NOT EXISTS tokens (
    token_id        TEXT PRIMARY KEY,  -- unique ID of the token
    token_type      TEXT CHECK (token_type IN ('RBT','FT','NFT','SC')),
    current_owner   TEXT,  -- DID currently owning it
    created_at      TIMESTAMPTZ 
);

CREATE TABLE IF NOT EXISTS allBlocks (
    block_id        TEXT PRIMARY KEY,  -- unique ID of the token
    block_type      TEXT CHECK (token_type IN ('')),   -- @check update the token type 
    epoch           TIMESTAMPTZ 
);

-- 2. Blocks Table
-- Each block = a transaction. Stores all transaction info.
CREATE TABLE IF NOT EXISTS blocks (
    block_id             TEXT PRIMARY KEY,
    prev_block_id        TEXT,         -- @check its an arry ? 
    sender_did           TEXT,
    receiver_did         TEXT,
    txn_type             TEXT,
    amount               DOUBLE PRECISION,
    txn_time             TIMESTAMP NOT NULL,
    epoch                BIGINT,
    time_taken_ms        BIGINT,
    tokens               TEXT[],       -- array of token strings
    validator_pledge_map JSONB         --@check JSON object: validator -> [pledgeToken1, pledgeToken2]
    txn_id               TEXT
);

-- 3. Token Chains Table
-- Maps blocks to each token's chain.
CREATE TABLE IF NOT EXISTS token_chains (
    token_id        TEXT REFERENCES tokens(token_id) ON DELETE CASCADE,
    block_height    BIGINT NOT NULL,
    block_id        TEXT REFERENCES blocks(block_id) ON DELETE CASCADE,
    PRIMARY KEY (token_id, block_height)
);

-- 4. DIDs Table
-- Stores all Decentralized Identifiers (users/wallets).
CREATE TABLE IF NOT EXISTS dids (
    did_id          TEXT PRIMARY KEY,
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    last_active     TIMESTAMPTZ,
    total_balance   NUMERIC(36,18) DEFAULT 0,  -- sum of tokens owned.   @check : how we can update it ? 
    pledged_amount  NUMERIC(36,18) DEFAULT 0
);

-- 5. Validators Table
-- Tracks validators in the network.
CREATE TABLE IF NOT EXISTS validators (
    validator_id    TEXT PRIMARY KEY,
    did_id          TEXT REFERENCES dids(did_id) ON DELETE CASCADE,
    active_since    TIMESTAMPTZ DEFAULT NOW(),
    last_seen       TIMESTAMPTZ,
    total_pledged   NUMERIC(36,18) DEFAULT 0,          -- @check how we are going to update this info ? 
    uptime_percent  NUMERIC(5,2) DEFAULT 100.0
);

-- 6. Pledges Table
-- Records tokens pledged by DIDs to validators.
CREATE TABLE IF NOT EXISTS pledges (
    pledge_id       SERIAL PRIMARY KEY,
    token_id        TEXT REFERENCES tokens(token_id) ON DELETE CASCADE,
    validator_id    TEXT REFERENCES validators(validator_id) ON DELETE CASCADE,
    did_id          TEXT REFERENCES dids(did_id) ON DELETE CASCADE,
    amount          NUMERIC(36,18) NOT NULL,
    pledged_at      TIMESTAMPTZ DEFAULT NOW()
);

-- 7. Token Stats / Aggregate Table (for homepage quick metrics)
CREATE TABLE IF NOT EXISTS token_stats (
    token_type      TEXT PRIMARY KEY,  -- RBT, FT, NFT, SC
    total_tokens    BIGINT DEFAULT 0,
    active_tokens   BIGINT DEFAULT 0,
    pledged_tokens  BIGINT DEFAULT 0,
    last_updated    TIMESTAMPTZ DEFAULT NOW()
);

-- 8. Transaction Graphs / Analytics Table
-- Pre-compute transaction counts by time intervals.
CREATE TABLE IF NOT EXISTS txn_analytics (
    interval_start  TIMESTAMPTZ,
    interval_end    TIMESTAMPTZ,
    txn_count       BIGINT,
    total_value     NUMERIC(36,18),
    token_type      TEXT,
    PRIMARY KEY (interval_start, interval_end, token_type)
);

-- 9. Smart Contracts Table
-- Stores deployed smart contracts and their metadata.
CREATE TABLE IF NOT EXISTS smart_contracts (
    contract_id     TEXT PRIMARY KEY,
    creator_did     TEXT REFERENCES dids(did_id) ON DELETE SET NULL,
    deployed_at     TIMESTAMPTZ DEFAULT NOW(),
    executor_did    TEXT REFERENCES dids(did_id) ON DELETE SET NULL,
    last_executed   TIMESTAMPTZ,
);

-- Create indexes for better performance
CREATE INDEX IF NOT EXISTS idx_tokens_type ON tokens(token_type);   --to fetch all the tokens of a particular type 
CREATE INDEX IF NOT EXISTS idx_tokens_owner ON tokens(current_owner);  -- to fetch balance of dids
CREATE INDEX IF NOT EXISTS idx_tokens_state ON tokens(state);  -- @check dont need it ???  

CREATE INDEX IF NOT EXISTS idx_blocks_hash ON blocks(block_hash);  -- @check dont need 
CREATE INDEX IF NOT EXISTS idx_blocks_sender ON blocks(sender_did);  
CREATE INDEX IF NOT EXISTS idx_blocks_receiver ON blocks(receiver_did);
CREATE INDEX IF NOT EXISTS idx_blocks_time ON blocks(txn_time); -- @check dont need it ???
CREATE INDEX IF NOT EXISTS idx_blocks_type ON blocks(txn_type); 

CREATE INDEX IF NOT EXISTS idx_token_chains_token ON token_chains(token_id);  --@check dont neet it
CREATE INDEX IF NOT EXISTS idx_token_chains_height ON token_chains(block_height);  --@check dont need it 

CREATE INDEX IF NOT EXISTS idx_dids_balance ON dids(total_balance);  --@check dont need it 
CREATE INDEX IF NOT EXISTS idx_dids_active ON dids(last_active);  

CREATE INDEX IF NOT EXISTS idx_validators_did ON validators(did_id);
CREATE INDEX IF NOT EXISTS idx_validators_active ON validators(active_since);

CREATE INDEX IF NOT EXISTS idx_pledges_token ON pledges(token_id); 
CREATE INDEX IF NOT EXISTS idx_pledges_validator ON pledges(validator_id); 
CREATE INDEX IF NOT EXISTS idx_pledges_did ON pledges(did_id);

CREATE INDEX IF NOT EXISTS idx_block_tokens_block ON block_tokens(block_id); -- @check we dont need this 
CREATE INDEX IF NOT EXISTS idx_block_tokens_token ON block_tokens(token_id); -- @check not needed 

CREATE INDEX IF NOT EXISTS idx_txn_analytics_time ON txn_analytics(interval_start, interval_end);
CREATE INDEX IF NOT EXISTS idx_txn_analytics_type ON txn_analytics(token_type); 


