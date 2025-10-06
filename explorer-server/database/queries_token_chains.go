package database

// GetTokenChain retrieves the blockchain for a specific token
func (bq *BlockQueries) GetTokenChain(tokenID string) ([]*TokenChain, error) {
	query := `
		SELECT tc.token_id, tc.block_height, tc.block_id
		FROM token_chains tc
		WHERE tc.token_id = $1
		ORDER BY tc.block_height ASC
	`
	rows, err := bq.db.Query(query, tokenID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chains []*TokenChain
	for rows.Next() {
		var chain TokenChain
		err := rows.Scan(
			&chain.TokenID, &chain.BlockHeight, &chain.BlockID,
		)
		if err != nil {
			return nil, err
		}
		chains = append(chains, &chain)
	}
	return chains, nil
}

// CreateTokenChain creates a new token chain entry
func (bq *BlockQueries) CreateTokenChain(tokenChain *TokenChain) error {
	query := `
		INSERT INTO token_chains (token_id, block_height, block_id)
		VALUES ($1, $2, $3)
		ON CONFLICT (token_id, block_height) DO NOTHING
	`
	_, err := bq.db.Exec(query,
		tokenChain.TokenID, tokenChain.BlockHeight, tokenChain.BlockID)
	return err
}