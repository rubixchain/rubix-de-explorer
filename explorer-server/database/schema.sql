-- Explorer Database Schema for Decentralized Token System
-- This file contains the schema for the decentralized explorer database

-- Create database if not exists (Note: This may need to be run separately)
-- CREATE DATABASE explorer_db;

-- Use the database
-- \c explorer_db;

-- 1. Tokens Table
-- Stores all tokens and their metadata.
CREATE TABLE IF NOT EXISTS tokens (
    token_id        TEXT PRIMARY KEY,  -- unique ID of the token
    token_type      TEXT CHECK (token_type IN ('RBT','FT','NFT','SC')),
    current_owner   TEXT,  -- DID currently owning it
    state           TEXT,  -- active, pledged, burned, etc.
    created_at      TIMESTAMPTZ DEFAULT NOW()
);

-- 2. Blocks Table
-- Each block = a transaction. Stores all transaction info.
CREATE TABLE IF NOT EXISTS blocks (
    block_id        TEXT PRIMARY KEY,   -- unique ID of block
    block_hash      TEXT UNIQUE NOT NULL,
    prev_block_hash TEXT,
    sender_did      TEXT,
    receiver_did    TEXT,
    txn_type        TEXT,  -- transfer, pledge, burn, etc.
    amount          NUMERIC(36,18), -- for FT
    txn_time        TIMESTAMPTZ NOT NULL,
    epoch           BIGINT,
    time_taken_ms   BIGINT
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
    total_balance   NUMERIC(36,18) DEFAULT 0,  -- sum of tokens owned
    pledged_amount  NUMERIC(36,18) DEFAULT 0
);

-- 5. Validators Table
-- Tracks validators in the network.
CREATE TABLE IF NOT EXISTS validators (
    validator_id    TEXT PRIMARY KEY,
    did_id          TEXT REFERENCES dids(did_id) ON DELETE CASCADE,
    active_since    TIMESTAMPTZ DEFAULT NOW(),
    last_seen       TIMESTAMPTZ,
    total_pledged   NUMERIC(36,18) DEFAULT 0,
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

-- 7. Transaction Tokens Table
-- If a block moves multiple tokens, map each token to the block.
CREATE TABLE IF NOT EXISTS block_tokens (
    block_id        TEXT REFERENCES blocks(block_id) ON DELETE CASCADE,
    token_id        TEXT REFERENCES tokens(token_id) ON DELETE CASCADE,
    amount          NUMERIC(36,18), -- for FT
    PRIMARY KEY (block_id, token_id)
);

-- 8. Token Stats / Aggregate Table (for homepage quick metrics)
CREATE TABLE IF NOT EXISTS token_stats (
    token_type      TEXT PRIMARY KEY,  -- RBT, FT, NFT, SC
    total_tokens    BIGINT DEFAULT 0,
    active_tokens   BIGINT DEFAULT 0,
    pledged_tokens  BIGINT DEFAULT 0,
    last_updated    TIMESTAMPTZ DEFAULT NOW()
);

-- 9. Transaction Graphs / Analytics Table
-- Pre-compute transaction counts by time intervals.
CREATE TABLE IF NOT EXISTS txn_analytics (
    interval_start  TIMESTAMPTZ,
    interval_end    TIMESTAMPTZ,
    txn_count       BIGINT,
    total_value     NUMERIC(36,18),
    token_type      TEXT,
    PRIMARY KEY (interval_start, interval_end, token_type)
);

-- Create indexes for better performance
CREATE INDEX IF NOT EXISTS idx_tokens_type ON tokens(token_type);
CREATE INDEX IF NOT EXISTS idx_tokens_owner ON tokens(current_owner);
CREATE INDEX IF NOT EXISTS idx_tokens_state ON tokens(state);

CREATE INDEX IF NOT EXISTS idx_blocks_hash ON blocks(block_hash);
CREATE INDEX IF NOT EXISTS idx_blocks_sender ON blocks(sender_did);
CREATE INDEX IF NOT EXISTS idx_blocks_receiver ON blocks(receiver_did);
CREATE INDEX IF NOT EXISTS idx_blocks_time ON blocks(txn_time);
CREATE INDEX IF NOT EXISTS idx_blocks_type ON blocks(txn_type);

CREATE INDEX IF NOT EXISTS idx_token_chains_token ON token_chains(token_id);
CREATE INDEX IF NOT EXISTS idx_token_chains_height ON token_chains(block_height);

CREATE INDEX IF NOT EXISTS idx_dids_balance ON dids(total_balance);
CREATE INDEX IF NOT EXISTS idx_dids_active ON dids(last_active);

CREATE INDEX IF NOT EXISTS idx_validators_did ON validators(did_id);
CREATE INDEX IF NOT EXISTS idx_validators_active ON validators(active_since);

CREATE INDEX IF NOT EXISTS idx_pledges_token ON pledges(token_id);
CREATE INDEX IF NOT EXISTS idx_pledges_validator ON pledges(validator_id);
CREATE INDEX IF NOT EXISTS idx_pledges_did ON pledges(did_id);

CREATE INDEX IF NOT EXISTS idx_block_tokens_block ON block_tokens(block_id);
CREATE INDEX IF NOT EXISTS idx_block_tokens_token ON block_tokens(token_id);

CREATE INDEX IF NOT EXISTS idx_txn_analytics_time ON txn_analytics(interval_start, interval_end);
CREATE INDEX IF NOT EXISTS idx_txn_analytics_type ON txn_analytics(token_type);