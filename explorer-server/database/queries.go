package database

import (
	"database/sql"
	"fmt"
	"log"
	"time"
)

// TokenQueries contains all token-related database operations
type TokenQueries struct {
	db *sql.DB
}

// BlockQueries contains all block-related database operations
type BlockQueries struct {
	db *sql.DB
}

// DIDQueries contains all DID-related database operations
type DIDQueries struct {
	db *sql.DB
}

// NewTokenQueries creates a new TokenQueries instance
func NewTokenQueries(db *sql.DB) *TokenQueries {
	return &TokenQueries{db: db}
}

// NewBlockQueries creates a new BlockQueries instance
func NewBlockQueries(db *sql.DB) *BlockQueries {
	return &BlockQueries{db: db}
}

// NewDIDQueries creates a new DIDQueries instance
func NewDIDQueries(db *sql.DB) *DIDQueries {
	return &DIDQueries{db: db}
}

// Token Operations

// CreateToken creates a new token
func (tq *TokenQueries) CreateToken(token *Token) error {
	query := `
		INSERT INTO tokens (token_id, token_type, current_owner, state)
		VALUES ($1, $2, $3, $4)
	`
	_, err := tq.db.Exec(query, token.TokenID, token.TokenType, token.CurrentOwner, token.State) // if the token is smart contract then there would't be any owner
	if err != nil {
		log.Printf("Error creating token: %v", err)
		return err
	}
	return nil
}

// GetToken retrieves a token by ID
func (tq *TokenQueries) GetToken(tokenID string) (*Token, error) {
	query := `
		SELECT token_id, token_type, current_owner, state, created_at
		FROM tokens
		WHERE token_id = $1
	`
	var token Token
	err := tq.db.QueryRow(query, tokenID).Scan(
		&token.TokenID, &token.TokenType, &token.CurrentOwner,
		&token.State, &token.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &token, nil
}

// GetTokensByOwner retrieves all tokens owned by a specific DID
func (tq *TokenQueries) GetTokensByOwner(ownerDID string) ([]*Token, error) {
	query := `
		SELECT token_id, token_type, current_owner, state, created_at
		FROM tokens
		WHERE current_owner = $1
		ORDER BY created_at DESC
	`
	rows, err := tq.db.Query(query, ownerDID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tokens []*Token
	for rows.Next() {
		var token Token
		err := rows.Scan(
			&token.TokenID, &token.TokenType, &token.CurrentOwner,
			&token.State, &token.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		tokens = append(tokens, &token)
	}
	return tokens, nil
}

// GetTokensByType retrieves all tokens of a specific type
func (tq *TokenQueries) GetTokensByType(tokenType string) ([]*Token, error) {
	query := `
		SELECT token_id, token_type, current_owner, state, created_at
		FROM tokens
		WHERE token_type = $1
		ORDER BY created_at DESC
	`
	rows, err := tq.db.Query(query, tokenType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tokens []*Token
	for rows.Next() {
		var token Token
		err := rows.Scan(
			&token.TokenID, &token.TokenType, &token.CurrentOwner,
			&token.State, &token.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		tokens = append(tokens, &token)
	}
	return tokens, nil
}

// UpdateTokenOwner updates the owner of a token
func (tq *TokenQueries) UpdateTokenOwner(tokenID, newOwner string) error {
	query := `
		UPDATE tokens
		SET current_owner = $1
		WHERE token_id = $2
	`
	result, err := tq.db.Exec(query, newOwner, tokenID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return fmt.Errorf("token not found: %s", tokenID)
	}

	return nil
}

// UpdateTokenState updates the state of a token
func (tq *TokenQueries) UpdateTokenState(tokenID, newState string) error {
	query := `
		UPDATE tokens
		SET state = $1
		WHERE token_id = $2
	`
	result, err := tq.db.Exec(query, newState, tokenID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return fmt.Errorf("token not found: %s", tokenID)
	}

	return nil
}

// Block Operations

// CreateBlock creates a new block
func (bq *BlockQueries) CreateBlock(block *Block) error {
	query := `
		INSERT INTO blocks (block_id, block_hash, prev_block_hash, sender_did, receiver_did, txn_type, amount, txn_time, epoch, time_taken_ms)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`
	_, err := bq.db.Exec(query,
		block.BlockID, block.PrevBlockID,
		block.SenderDID, block.ReceiverDID, block.TxnType,
		block.Amount, block.TxnTime, block.Epoch, block.TimeTakenMs,
	)
	if err != nil {
		log.Printf("Error creating block: %v", err)
		return err
	}
	return nil
}

// GetBlock retrieves a block by ID
func (bq *BlockQueries) GetBlock(blockID string) (*Block, error) {
	query := `
		SELECT block_id, block_hash, prev_block_hash, sender_did, receiver_did, txn_type, amount, txn_time, epoch, time_taken_ms
		FROM blocks
		WHERE block_id = $1
	`
	var block Block
	err := bq.db.QueryRow(query, blockID).Scan(
		&block.BlockID,&block.PrevBlockID,
		&block.SenderDID, &block.ReceiverDID, &block.TxnType,
		&block.Amount, &block.TxnTime, &block.Epoch, &block.TimeTakenMs, &block.Tokens, 
		&block.
	)
	if err != nil {
		return nil, err
	}
	return &block, nil
}


// GetLatestBlocks retrieves the latest blocks
func (bq *BlockQueries) GetLatestBlocks(limit int) ([]*Block, error) {
	query := `
		SELECT block_id, block_hash, prev_block_id, sender_did, receiver_did, txn_type, amount, txn_time, epoch, time_taken_ms, tokens, validator_pledge_map, txn_id
		FROM blocks
		ORDER BY txn_time DESC
		LIMIT $1
	`
	rows, err := bq.db.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var blocks []*Block
	for rows.Next() {
		var block Block
		err := rows.Scan(
			&block.BlockID, &block.BlockID, &block.PrevBlockID,
			&block.SenderDID, &block.ReceiverDID, &block.TxnType,
			&block.Amount, &block.TxnTime, &block.Epoch, &block.TimeTakenMs,
			&block.Tokens, &block.ValidatorPledgeMap, &block.TxnID,
		)
		if err != nil {
			return nil, err
		}
		blocks = append(blocks, &block)
	}
	return blocks, nil
}

// GetBlocksByDID retrieves all blocks associated with a DID (as sender or receiver)
func (bq *BlockQueries) GetBlocksByDID(didID string) ([]*Block, error) {
	query := `
		SELECT block_id, block_hash, prev_block_hash, sender_did, receiver_did, txn_type, amount, txn_time, epoch, time_taken_ms
		FROM blocks
		WHERE sender_did = $1 OR receiver_did = $1
		ORDER BY txn_time DESC
	`
	rows, err := bq.db.Query(query, didID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var blocks []*Block
	for rows.Next() {
		var block Block
		err := rows.Scan(
			&block.BlockID, &block.BlockHash, &block.PrevBlockHash,
			&block.SenderDID, &block.ReceiverDID, &block.TxnType,
			&block.Amount, &block.TxnTime, &block.Epoch, &block.TimeTakenMs,
		)
		if err != nil {
			return nil, err
		}
		blocks = append(blocks, &block)
	}
	return blocks, nil
}

// DID Operations

// CreateDID creates a new DID
func (dq *DIDQueries) CreateDID(did *DID) error {
	query := `
		INSERT INTO dids (did_id, last_active, total_balance, pledged_amount)
		VALUES ($1, $2, $3, $4)
	`
	_, err := dq.db.Exec(query, did.DIDID, did.LastActive, did.TotalBalance, did.PledgedAmount)
	if err != nil {
		log.Printf("Error creating DID: %v", err)
		return err
	}
	return nil
}

// GetDID retrieves a DID by ID
func (dq *DIDQueries) GetDID(didID string) (*DID, error) {
	query := `
		SELECT did_id, created_at, last_active, total_balance, pledged_amount
		FROM dids
		WHERE did_id = $1
	`
	var did DID
	err := dq.db.QueryRow(query, didID).Scan(
		&did.DIDID, &did.CreatedAt, &did.LastActive,
		&did.TotalBalance, &did.PledgedAmount,
	)
	if err != nil {
		return nil, err
	}
	return &did, nil
}

// UpdateDIDActivity updates the last activity time for a DID
func (dq *DIDQueries) UpdateDIDActivity(didID string) error {
	query := `
		UPDATE dids
		SET last_active = $1
		WHERE did_id = $2
	`
	_, err := dq.db.Exec(query, time.Now(), didID)
	return err
}

// UpdateDIDBalance updates the total balance for a DID
func (dq *DIDQueries) UpdateDIDBalance(didID string, balance float64) error {
	query := `
		UPDATE dids
		SET total_balance = $1
		WHERE did_id = $2
	`
	_, err := dq.db.Exec(query, balance, didID)
	return err
}

// GetTokenStats retrieves token statistics
func GetTokenStats() ([]*TokenStats, error) {
	query := `
		SELECT
			token_type,
			COUNT(*) as total_tokens,
			COUNT(CASE WHEN state = 'active' THEN 1 END) as active_tokens,
			COUNT(CASE WHEN state = 'pledged' THEN 1 END) as pledged_tokens
		FROM tokens
		GROUP BY token_type
	`
	rows, err := DB.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []*TokenStats
	for rows.Next() {
		var stat TokenStats
		err := rows.Scan(&stat.TokenType, &stat.TotalTokens, &stat.ActiveTokens, &stat.PledgedTokens)
		if err != nil {
			return nil, err
		}
		stats = append(stats, &stat)
	}
	return stats, nil
}

// GetRecentTransactionStats retrieves recent transaction statistics
func GetRecentTransactionStats(hours int) (*TxnAnalytics, error) {
	query := `
		SELECT
			$1::timestamptz as interval_start,
			NOW() as interval_end,
			COUNT(*) as txn_count,
			COALESCE(SUM(amount), 0) as total_value
		FROM blocks
		WHERE txn_time >= NOW() - INTERVAL '%d hours'
	`
	var analytics TxnAnalytics
	startTime := time.Now().Add(-time.Duration(hours) * time.Hour)

	err := DB.QueryRow(fmt.Sprintf(query, hours), startTime).Scan(
		&analytics.IntervalStart, &analytics.IntervalEnd,
		&analytics.TxnCount, &analytics.TotalValue,
	)
	if err != nil {
		return nil, err
	}
	return &analytics, nil
}

// Token Chain Operations

// CreateTokenChain creates a new token chain entry
func CreateTokenChain(tokenChain *TokenChain) error {
	query := `
		INSERT INTO token_chains (token_id, block_height, block_id)
		VALUES ($1, $2, $3)
		ON CONFLICT (token_id, block_height) DO NOTHING
	`
	_, err := DB.Exec(query, tokenChain.TokenID, tokenChain.BlockHeight, tokenChain.BlockID)
	if err != nil {
		log.Printf("Error creating token chain: %v", err)
		return err
	}
	return nil
}

// GetTokenChain retrieves the chain for a specific token
func GetTokenChain(tokenID string) ([]*TokenChain, error) {
	query := `
		SELECT token_id, block_height, block_id
		FROM token_chains
		WHERE token_id = $1
		ORDER BY block_height ASC
	`
	rows, err := DB.Query(query, tokenID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chains []*TokenChain
	for rows.Next() {
		var chain TokenChain
		err := rows.Scan(&chain.TokenID, &chain.BlockHeight, &chain.BlockID)
		if err != nil {
			return nil, err
		}
		chains = append(chains, &chain)
	}
	return chains, nil
}

// Block Token Operations

// CreateBlockToken creates a new block token mapping
func CreateBlockToken(blockToken *BlockToken) error {
	query := `
		INSERT INTO block_tokens (block_id, token_id, amount)
		VALUES ($1, $2, $3)
		ON CONFLICT (block_id, token_id) DO NOTHING
	`
	_, err := DB.Exec(query, blockToken.BlockID, blockToken.TokenID, blockToken.Amount)
	if err != nil {
		log.Printf("Error creating block token: %v", err)
		return err
	}
	return nil
}

// GetBlockTokens retrieves all tokens associated with a block
func GetBlockTokens(blockID string) ([]*BlockToken, error) {
	query := `
		SELECT block_id, token_id, amount
		FROM block_tokens
		WHERE block_id = $1
	`
	rows, err := DB.Query(query, blockID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var blockTokens []*BlockToken
	for rows.Next() {
		var bt BlockToken
		err := rows.Scan(&bt.BlockID, &bt.TokenID, &bt.Amount)
		if err != nil {
			return nil, err
		}
		blockTokens = append(blockTokens, &bt)
	}
	return blockTokens, nil
}

// Batch Operations for Performance

// BatchCreateTokens creates multiple tokens in a single transaction
func BatchCreateTokens(tokens []*Token) error {
	if len(tokens) == 0 {
		return nil
	}

	tx, err := DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO tokens (token_id, token_type, current_owner, state)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (token_id) DO UPDATE SET
			current_owner = EXCLUDED.current_owner,
			state = EXCLUDED.state
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, token := range tokens {
		_, err := stmt.Exec(token.TokenID, token.TokenType, token.CurrentOwner, token.State)
		if err != nil {
			log.Printf("Error batch creating token %s: %v", token.TokenID, err)
			return err
		}
	}

	return tx.Commit()
}

// BatchCreateBlocks creates multiple blocks in a single transaction
func BatchCreateBlocks(blocks []*Block) error {
	if len(blocks) == 0 {
		return nil
	}

	tx, err := DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO blocks (block_id, block_hash, prev_block_hash, sender_did, receiver_did, txn_type, amount, txn_time, epoch, time_taken_ms)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (block_id) DO NOTHING
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, block := range blocks {
		_, err := stmt.Exec(
			block.BlockID, block.BlockHash, block.PrevBlockHash,
			block.SenderDID, block.ReceiverDID, block.TxnType,
			block.Amount, block.TxnTime, block.Epoch, block.TimeTakenMs,
		)
		if err != nil {
			log.Printf("Error batch creating block %s: %v", block.BlockID, err)
			return err
		}
	}

	return tx.Commit()
}

// Search and Filter Operations

// SearchTokens searches tokens by various criteria
func SearchTokens(tokenType, ownerDID, state string, limit, offset int) ([]*Token, error) {
	query := `
		SELECT token_id, token_type, current_owner, state, created_at
		FROM tokens
		WHERE 1=1
	`
	args := []interface{}{}
	argCount := 0

	if tokenType != "" {
		argCount++
		query += fmt.Sprintf(" AND token_type = $%d", argCount)
		args = append(args, tokenType)
	}

	if ownerDID != "" {
		argCount++
		query += fmt.Sprintf(" AND current_owner = $%d", argCount)
		args = append(args, ownerDID)
	}

	if state != "" {
		argCount++
		query += fmt.Sprintf(" AND state = $%d", argCount)
		args = append(args, state)
	}

	query += " ORDER BY created_at DESC"

	if limit > 0 {
		argCount++
		query += fmt.Sprintf(" LIMIT $%d", argCount)
		args = append(args, limit)
	}

	if offset > 0 {
		argCount++
		query += fmt.Sprintf(" OFFSET $%d", argCount)
		args = append(args, offset)
	}

	rows, err := DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tokens []*Token
	for rows.Next() {
		var token Token
		err := rows.Scan(
			&token.TokenID, &token.TokenType, &token.CurrentOwner,
			&token.State, &token.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		tokens = append(tokens, &token)
	}
	return tokens, nil
}

// SearchBlocks searches blocks by various criteria
func SearchBlocks(senderDID, receiverDID, txnType string, limit, offset int) ([]*Block, error) {
	query := `
		SELECT block_id, block_hash, prev_block_hash, sender_did, receiver_did, txn_type, amount, txn_time, epoch, time_taken_ms
		FROM blocks
		WHERE 1=1
	`
	args := []interface{}{}
	argCount := 0

	if senderDID != "" {
		argCount++
		query += fmt.Sprintf(" AND sender_did = $%d", argCount)
		args = append(args, senderDID)
	}

	if receiverDID != "" {
		argCount++
		query += fmt.Sprintf(" AND receiver_did = $%d", argCount)
		args = append(args, receiverDID)
	}

	if txnType != "" {
		argCount++
		query += fmt.Sprintf(" AND txn_type = $%d", argCount)
		args = append(args, txnType)
	}

	query += " ORDER BY txn_time DESC"

	if limit > 0 {
		argCount++
		query += fmt.Sprintf(" LIMIT $%d", argCount)
		args = append(args, limit)
	}

	if offset > 0 {
		argCount++
		query += fmt.Sprintf(" OFFSET $%d", argCount)
		args = append(args, offset)
	}

	rows, err := DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var blocks []*Block
	for rows.Next() {
		var block Block
		err := rows.Scan(
			&block.BlockID, &block.BlockHash, &block.PrevBlockHash,
			&block.SenderDID, &block.ReceiverDID, &block.TxnType,
			&block.Amount, &block.TxnTime, &block.Epoch, &block.TimeTakenMs,
		)
		if err != nil {
			return nil, err
		}
		blocks = append(blocks, &block)
	}
	return blocks, nil
}

// get all asset counts
func GetAllAssetCounts() (int, error) {
	query := `
		SELECT COUNT(*) FROM tokens
	`
	var count int
	err := DB.QueryRow(query).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

// get all did count
func GetAllDIDCount() (int, error) {
	var count int
	err := DB.QueryRow("SELECT COUNT(*) FROM dids").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get DID count: %v", err)
	}
	return count, nil
}

// get the did with most rbts
func GetTopDIDsWithMostRBTs() ([]struct {
	DIDID    string
	RBTCount int
}, error) {
	query := `
		SELECT current_owner, COUNT(*) as rbt_count
		FROM tokens
		WHERE token_type = 'RBT'
		GROUP BY current_owner
		ORDER BY rbt_count DESC
		LIMIT $10
	`
	rows, err := DB.Query(query, 10)
	if err != nil {
		return nil, fmt.Errorf("failed to query top DIDs: %v", err)
	}
	defer rows.Close()
	var results []struct {
		DIDID    string
		RBTCount int
	}
	for rows.Next() {
		var didID string
		var rbtCount int
		if err := rows.Scan(&didID, &rbtCount); err != nil {
			return nil, fmt.Errorf("failed to scan row: %v", err)
		}
		results = append(results, struct {
			DIDID    string
			RBTCount int
		}{
			DIDID:    didID,
			RBTCount: rbtCount,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error: %v", err)
	}
	return results, nil
}
