-- =========================================================================
-- Rubix Explorer Modern Schema (7-Table Consolidated Architecture)
-- Sync Date: 2026-03-26
-- Matches: database/models/models.go
-- =========================================================================

-- 1. Transactions: Raw transaction storage
CREATE TABLE IF NOT EXISTS "Transactions" (
    "id" TEXT PRIMARY KEY,
    "info" JSONB,
    "signature" JSONB,
    "created_at" TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- 2. EventTransactions: PubSub status tracking
CREATE TABLE IF NOT EXISTS "EventTransactions" (
    "transaction_id" TEXT PRIMARY KEY,
    "status" BOOLEAN DEFAULT FALSE,
    "message" TEXT,
    "created_at" TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- 3. TransactionInfo: Flattened details for fast querying
CREATE TABLE IF NOT EXISTS "TransactionInfo" (
    "transaction_id" TEXT PRIMARY KEY,
    "initiator" TEXT,
    "owner" TEXT,
    "epoch" INTEGER,
    "network" TEXT,
    "tokens" JSONB,
    "committed_tokens" JSONB,
    "quorums" JSONB,
    "memo" TEXT,
    "created_at" TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_txn_info_initiator ON "TransactionInfo"(initiator);
CREATE INDEX IF NOT EXISTS idx_txn_info_owner ON "TransactionInfo"(owner);
CREATE INDEX IF NOT EXISTS idx_txn_info_epoch ON "TransactionInfo"(epoch DESC);

-- 4. Tokens: Unified state for RBT, FT, NFT, and SC
CREATE TABLE IF NOT EXISTS "Tokens" (
    "token_id" TEXT PRIMARY KEY,
    "parent_token_id" TEXT,
    "token_value" DOUBLE PRECISION DEFAULT 0,
    "token_status" SMALLINT DEFAULT 1,
    "did" TEXT, -- Current Owner (or Initiator for SCs)
    "transaction_id" TEXT, -- Latest Transaction ID
    "token_state_hash" TEXT,
    "token_type" SMALLINT, -- 1:RBT, 2:FT, 3:NFT, 4:SC
    "latest_position" BIGINT DEFAULT 0,
    "latest_role" SMALLINT DEFAULT 0,
    "data" TEXT, -- Metadata or SC payload
    "deployer_did" TEXT, -- Original creator/deployer
    "needs_sync" BOOLEAN DEFAULT FALSE,
    "created_at" TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_tokens_did ON "Tokens"(did);
CREATE INDEX IF NOT EXISTS idx_tokens_type ON "Tokens"(token_type);
CREATE INDEX IF NOT EXISTS idx_tokens_deployer ON "Tokens"(deployer_did);

-- 5. TokenChain: Granular provenance tracking
CREATE TABLE IF NOT EXISTS "TokenChain" (
    "id" BIGSERIAL PRIMARY KEY,
    "token_id" TEXT,
    "transaction_id" TEXT,
    "role" SMALLINT,
    "previous_transaction_id" TEXT,
    "created_at" TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_tokenchain_token_id ON "TokenChain"(token_id);

-- 6. TokenChainArray: Optimized history sequence
CREATE TABLE IF NOT EXISTS "TokenChainArray" (
    "token_id" TEXT PRIMARY KEY,
    "index" JSONB, -- Array of TokenChain internal IDs
    "created_at" TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- 7. DIDBalances: Analytics and aggregation
CREATE TABLE IF NOT EXISTS "DIDBalances" (
    "did" TEXT,
    "asset_type" TEXT,
    "token_name" TEXT,
    "creator_did" TEXT DEFAULT '', -- FT creator DID (empty for RBT/NFT/SC)
    "balance" DOUBLE PRECISION DEFAULT 0,
    "last_update" BIGINT,
    PRIMARY KEY ("did", "asset_type", "token_name", "creator_did")
);

CREATE INDEX IF NOT EXISTS idx_did_balances_asset ON "DIDBalances"("asset_type", "token_name");
CREATE INDEX IF NOT EXISTS idx_did_balances_rank ON "DIDBalances"("balance" DESC);
