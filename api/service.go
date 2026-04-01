package api

import (
	"encoding/json"
	"explorer-server/database"
	"explorer-server/database/models"
	"explorer-server/model"
	"fmt"
	"strings"
)

// SearchResult is the unified structure for search API responses
type SearchResult struct {
	Type string      `json:"type"` // "DID", "Token", "Transaction"
	Data interface{} `json:"data"`
}

// -------------------------------------------------------------------
// 1. Search Logic
// -------------------------------------------------------------------

// GetSearchInfo routes search queries to the appropriate table based on format
func GetSearchInfo(query string) (*SearchResult, error) {
	if query == "" {
		return nil, fmt.Errorf("empty query")
	}

	// 1. DID Search (starts with bafybm)
	if strings.HasPrefix(query, "bafybm") {
		var balances []models.DIDBalance
		if err := database.ReadDB.Table("DIDBalances").Where("did = ?", query).Find(&balances).Error; err == nil && len(balances) > 0 {
			return &SearchResult{Type: "DID", Data: balances}, nil
		}
	}

	// 2. Transaction Search (prioritize DB check to avoid ID collisions)
	var txn models.TransactionInfo
	if err := database.ReadDB.Table("TransactionInfo").Where("transaction_id = ?", query).First(&txn).Error; err == nil {
		return &SearchResult{Type: "Transaction", Data: txn}, nil
	}

	// 3. Token Search (starts with Qm or contains _)
	if strings.HasPrefix(query, "Qm") || strings.Contains(query, "_") {
		var token models.Token
		if err := database.ReadDB.Table("Tokens").Where("token_id = ?", query).First(&token).Error; err == nil {
			return &SearchResult{Type: "Token", Data: token}, nil
		}
	}

	return nil, fmt.Errorf("no data found for ID: %s", query)
}

// -------------------------------------------------------------------
// 2. Statistics (Counts)
// -------------------------------------------------------------------

func GetRBTCount() (int64, error) {
	var count int64
	if err := database.ReadDB.Model(&models.Token{}).Where("token_type = ?", 1).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func GetFTCount() (int64, error) {
	var count int64
	if err := database.ReadDB.Model(&models.Token{}).Where("token_type = ?", 2).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func GetNFTCount() (int64, error) {
	var count int64
	if err := database.ReadDB.Model(&models.Token{}).Where("token_type = ?", 3).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func GetSCCount() (int64, error) {
	var count int64
	if err := database.ReadDB.Model(&models.Token{}).Where("token_type = ?", 4).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func GetTxnsCount() (int64, error) {
	var count int64
	if err := database.ReadDB.Model(&models.Transactions{}).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func GetDIDCount() (int64, error) {
	var count int64
	if err := database.ReadDB.Model(&models.DIDBalance{}).Distinct("did").Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// -------------------------------------------------------------------
// 3. Lists and Holders
// -------------------------------------------------------------------

// GetDAGFromTxn returns the anchor transaction plus up to `depth` levels of ancestors,
// traversing the TokenChain backwards via previous_transaction_id.
// Nodes are all unique TransactionInfo records. Edges are directed child → parent links.
func GetDAGFromTxn(txnID string, depth int) (model.DAGResponse, error) {
	if depth <= 0 || depth > 100 {
		depth = 100
	}

	// Recursive CTE: walk backwards through TokenChain up to `depth` hops.
	// UNION (not UNION ALL) deduplicates rows and prevents cycles.
	type edgeRow struct {
		ChildTxnID  string `gorm:"column:child_txn_id"`
		ParentTxnID string `gorm:"column:parent_txn_id"`
	}
	var edges []edgeRow
	if err := database.ReadDB.Raw(`
		WITH RECURSIVE dag_edges AS (
			SELECT DISTINCT
				transaction_id          AS child_txn_id,
				previous_transaction_id AS parent_txn_id,
				1                       AS depth
			FROM "TokenChain"
			WHERE transaction_id = ?
			  AND previous_transaction_id IS NOT NULL
			  AND previous_transaction_id <> ''

			UNION

			SELECT
				tc.transaction_id,
				tc.previous_transaction_id,
				de.depth + 1
			FROM "TokenChain" tc
			INNER JOIN dag_edges de ON tc.transaction_id = de.parent_txn_id
			WHERE de.depth < ?
			  AND tc.previous_transaction_id IS NOT NULL
			  AND tc.previous_transaction_id <> ''
		)
		SELECT DISTINCT child_txn_id, parent_txn_id FROM dag_edges
	`, txnID, depth).Scan(&edges).Error; err != nil {
		return model.DAGResponse{}, err
	}

	// Collect all unique txnIDs (anchor + all nodes from edges).
	seen := map[string]struct{}{txnID: {}}
	for _, e := range edges {
		seen[e.ChildTxnID] = struct{}{}
		seen[e.ParentTxnID] = struct{}{}
	}
	txnIDs := make([]string, 0, len(seen))
	for id := range seen {
		txnIDs = append(txnIDs, id)
	}

	// Fetch TransactionInfo for all collected txnIDs.
	var txns []models.TransactionInfo
	if err := database.ReadDB.Table("TransactionInfo").
		Where("transaction_id IN ?", txnIDs).
		Order("epoch DESC").
		Find(&txns).Error; err != nil {
		return model.DAGResponse{}, err
	}

	// Build the DAGEdge slice for the response.
	dagEdges := make([]model.DAGEdge, len(edges))
	for i, e := range edges {
		dagEdges[i] = model.DAGEdge{From: e.ChildTxnID, To: e.ParentTxnID}
	}

	if txns == nil {
		txns = []models.TransactionInfo{}
	}
	return model.DAGResponse{Transactions: txns, Edges: dagEdges}, nil
}

// GetDAGTransactions returns the latest 1000 transactions ordered by epoch descending.
func GetDAGTransactions() ([]models.TransactionInfo, error) {
	var transactions []models.TransactionInfo
	if err := database.ReadDB.Table("TransactionInfo").Order("epoch DESC").Limit(1000).Find(&transactions).Error; err != nil {
		return nil, err
	}
	return transactions, nil
}

func GetTransactionInfoList(limit, page int) ([]models.TransactionInfo, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 10
	}
	offset := (page - 1) * limit

	// 1. Fetch from EventTransactions to get ordered list of TxnIDs and their status
	var events []models.EventTransaction
	if err := database.ReadDB.Table("EventTransactions").Order("created_at DESC").Limit(limit).Offset(offset).Find(&events).Error; err != nil {
		return nil, err
	}

	result := make([]models.TransactionInfo, 0)
	for _, event := range events {
		var info models.TransactionInfo
		if event.Status {
			// Fetch from Success table
			if err := database.ReadDB.Table("TransactionInfo").Where("transaction_id = ?", event.TransactionID).First(&info).Error; err == nil {
				info.Status = true
				result = append(result, info)
			}
		} else {
			// Fetch from Failure table
			var failed models.FailedTransactionInfo
			if err := database.ReadDB.Table("FailedTransactionInfo").Where("transaction_id = ?", event.TransactionID).First(&failed).Error; err == nil {
				// Convert FailedTransactionInfo to TransactionInfo for consistent API response
				info = models.TransactionInfo{
					TransactionID:   failed.TransactionID,
					Initiator:       failed.Initiator,
					Owner:           failed.Owner,
					Epoch:           failed.Epoch,
					Network:         failed.Network,
					Tokens:          failed.Tokens,
					CommittedTokens: failed.CommittedTokens,
					Quorums:         failed.Quorums,
					Memo:            failed.Memo,
					Status:          false,
					Amount:          failed.Amount,
					CreatedAt:       failed.CreatedAt,
					UpdatedAt:       failed.UpdatedAt,
				}
				result = append(result, info)
			}
		}
	}

	return result, nil
}

func GetDIDHoldersList(limit, page int) ([]models.DIDBalance, error) {
	var balances []models.DIDBalance
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 10
	}
	offset := (page - 1) * limit

	if err := database.ReadDB.Table("DIDBalances").Where("asset_type = ? AND balance > 0", "RBT").Order("balance DESC").Limit(limit).Offset(offset).Find(&balances).Error; err != nil {
		return nil, err
	}
	return balances, nil
}

func GetDIDBalance(did string) ([]models.DIDBalance, error) {
	var balances []models.DIDBalance
	if err := database.ReadDB.Table("DIDBalances").Where("did = ?", did).Find(&balances).Error; err != nil {
		return nil, err
	}
	return balances, nil
}

func GetRBTList(limit, page int) ([]models.Token, error) {
	var tokens []models.Token
	offset := (page - 1) * limit

	if err := database.ReadDB.Model(&models.Token{}).Where("token_type = ?", 1).Order("updated_at DESC").Limit(limit).Offset(offset).Find(&tokens).Error; err != nil {
		return nil, err
	}
	return tokens, nil
}

func GetFTGroupList(limit, page int) ([]model.FTGroup, error) {
	var tokens []models.Token
	if err := database.ReadDB.Model(&models.Token{}).Where("token_type = ?", 2).Find(&tokens).Error; err != nil {
		return nil, err
	}

	groupsMap := make(map[string]*model.FTGroup)
	for _, t := range tokens {
		parts := strings.Split(t.TokenID, "_")
		if len(parts) >= 3 {
			ftName := parts[0]
			creatorDID := parts[len(parts)-1]
			key := ftName + "_" + creatorDID
			if g, ok := groupsMap[key]; ok {
				g.Count++
			} else {
				groupsMap[key] = &model.FTGroup{FTName: ftName, Count: 1, CreatorDID: creatorDID}
			}
		}
	}

	var allGroups []model.FTGroup
	for _, g := range groupsMap {
		allGroups = append(allGroups, *g)
	}

	start := (page - 1) * limit
	if start > len(allGroups) {
		start = len(allGroups)
	}
	end := start + limit
	if end > len(allGroups) {
		end = len(allGroups)
	}
	return allGroups[start:end], nil
}

func GetFTListByFTName(ftName string, creatorDID string, limit, page int) ([]models.Token, error) {
	var tokens []models.Token
	offset := (page - 1) * limit
	query := database.ReadDB.Model(&models.Token{}).Where("token_type = ?", 2).Where("token_id LIKE ?", ftName+"_%")
	if creatorDID != "" {
		query = query.Where("token_id LIKE ?", "%_"+creatorDID)
	}

	if err := query.Order("updated_at DESC").Limit(limit).Offset(offset).Find(&tokens).Error; err != nil {
		return nil, err
	}
	return tokens, nil
}

func GetSCList(limit, page int) ([]models.Token, error) {
	var tokens []models.Token
	offset := (page - 1) * limit

	if err := database.ReadDB.Model(&models.Token{}).Where("token_type = ?", 4).Order("updated_at DESC").Limit(limit).Offset(offset).Find(&tokens).Error; err != nil {
		return nil, err
	}
	return tokens, nil
}

// -------------------------------------------------------------------
// 4. Specific Info and History
// -------------------------------------------------------------------

func GetTransactionInfo(txnID string) (models.TransactionInfo, error) {
	var transaction models.TransactionInfo
	if err := database.ReadDB.Table("TransactionInfo").Where("transaction_id = ?", txnID).First(&transaction).Error; err != nil {
		return transaction, err
	}
	return transaction, nil
}

func GetTxnsByDID(did string, page, limit int) ([]models.TransactionInfo, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 10
	}
	offset := (page - 1) * limit

	var transactions []models.TransactionInfo
	if err := database.ReadDB.Table("TransactionInfo").
		Where("initiator = ? OR owner = ?", did, did).
		Order("epoch DESC").
		Limit(limit).Offset(offset).
		Find(&transactions).Error; err != nil {
		return nil, err
	}
	return transactions, nil
}

func GetTransactionInfoListByToken(tokenID string, page, limit int) ([]models.TransactionInfo, error) {
	ids, err := GetTransactionIDList(tokenID, page, limit)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return []models.TransactionInfo{}, nil
	}

	txnIDs := make([]string, len(ids))
	for i, entry := range ids {
		txnIDs[i] = entry.TransactionID
	}

	var transactions []models.TransactionInfo
	if err := database.ReadDB.Table("TransactionInfo").Where("transaction_id IN ?", txnIDs).Order("epoch DESC").Find(&transactions).Error; err != nil {
		return nil, err
	}
	return transactions, nil
}

func GetTransactionIDList(tokenID string, page, limit int) ([]models.TokenChain, error) {
	var tca models.TokenChainArray
	if err := database.ReadDB.Table("TokenChainArray").Where("token_id = ?", tokenID).First(&tca).Error; err != nil {
		return nil, err
	}

	var chain []uint64
	if err := json.Unmarshal(tca.Index, &chain); err != nil {
		return nil, err
	}

	total := len(chain)
	if total == 0 {
		return []models.TokenChain{}, nil
	}

	end := total - ((page - 1) * limit)
	start := total - (page * limit)
	if end <= 0 {
		return []models.TokenChain{}, nil
	}
	if start < 0 {
		start = 0
	}
	pagedIndices := chain[start:end]

	var history []models.TokenChain
	if err := database.ReadDB.Table("TokenChain").Where("id IN ?", pagedIndices).Order("id DESC").Find(&history).Error; err != nil {
		return nil, err
	}
	return history, nil
}

func GetTokenInfo(tokenID string) (models.Token, error) {
	var token models.Token
	if err := database.ReadDB.Table("Tokens").Where("token_id = ?", tokenID).First(&token).Error; err != nil {
		return token, err
	}
	return token, nil
}

// SearchRBTSuggestions returns token IDs for RBT tokens whose ID starts with the given prefix.
func SearchRBTSuggestions(prefix string, limit int) ([]model.RBTSuggestion, error) {
	if limit <= 0 || limit > 20 {
		limit = 10
	}
	var suggestions []model.RBTSuggestion
	err := database.ReadDB.Raw(`
		SELECT token_id
		FROM "Tokens"
		WHERE token_type = 1
			AND token_id LIKE ?
		ORDER BY token_id
		LIMIT ?
	`, prefix+"%", limit).Scan(&suggestions).Error
	return suggestions, err
}

// GetRBTInfo returns owner and value for a single RBT token.
func GetRBTInfo(tokenID string) (model.RBTInfo, error) {
	var token models.Token
	if err := database.ReadDB.Table("Tokens").
		Where("token_id = ? AND token_type = ?", tokenID, 1).
		First(&token).Error; err != nil {
		return model.RBTInfo{}, err
	}
	return model.RBTInfo{
		TokenID:    token.TokenID,
		OwnerDID:   token.DID,
		TokenValue: token.TokenValue,
	}, nil
}

// GetFTInfo returns aggregate details for a specific FT (identified by name + creator DID).
func GetFTInfo(ftName, creatorDID string) (model.FTInfo, error) {
	var info model.FTInfo
	err := database.ReadDB.Raw(`
		SELECT
			split_part(token_id, '_', 1)                              AS ft_name,
			reverse(split_part(reverse(token_id), '_', 1))            AS creator_did,
			MAX(token_value)                                           AS ft_value,
			COUNT(*)                                                   AS total_amount,
			EXTRACT(EPOCH FROM MIN(created_at))::bigint                AS created_time
		FROM "Tokens"
		WHERE token_type = 2
			AND split_part(token_id, '_', 1) = ?
			AND reverse(split_part(reverse(token_id), '_', 1)) = ?
		GROUP BY ft_name, creator_did
	`, ftName, creatorDID).Scan(&info).Error
	return info, err
}

// GetFTTopHolders returns the top holders (by token count) for a specific FT,
// identified by its name and creator DID, in descending order with pagination.
func GetFTTopHolders(ftName, creatorDID string, limit, page int) (model.FTTopHoldersResponse, error) {
	if limit <= 0 {
		limit = 10
	}
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * limit

	var holders []model.FTHolder
	err := database.ReadDB.Raw(`
		SELECT did, COUNT(*) AS token_count
		FROM "Tokens"
		WHERE token_type = 2
			AND split_part(token_id, '_', 1) = ?
			AND reverse(split_part(reverse(token_id), '_', 1)) = ?
		GROUP BY did
		ORDER BY token_count DESC
		LIMIT ? OFFSET ?
	`, ftName, creatorDID, limit, offset).Scan(&holders).Error
	if err != nil {
		return model.FTTopHoldersResponse{}, err
	}

	var countResult struct{ Count int64 }
	if err := database.ReadDB.Raw(`
		SELECT COUNT(DISTINCT did) AS count
		FROM "Tokens"
		WHERE token_type = 2
			AND split_part(token_id, '_', 1) = ?
			AND reverse(split_part(reverse(token_id), '_', 1)) = ?
	`, ftName, creatorDID).Scan(&countResult).Error; err != nil {
		return model.FTTopHoldersResponse{}, err
	}

	if holders == nil {
		holders = []model.FTHolder{}
	}
	return model.FTTopHoldersResponse{
		Holders:    holders,
		TotalCount: countResult.Count,
		Page:       page,
		Limit:      limit,
	}, nil
}

// SearchFTSuggestions returns distinct (ft_name, creator_did) pairs where
// the FT name starts with the given prefix (case-insensitive). Used for autocomplete.
func SearchFTSuggestions(prefix string, limit int) ([]model.FTSuggestion, error) {
	if limit <= 0 || limit > 20 {
		limit = 10
	}
	var suggestions []model.FTSuggestion
	err := database.ReadDB.Raw(`
		SELECT DISTINCT
			split_part(token_id, '_', 1) AS ft_name,
			reverse(split_part(reverse(token_id), '_', 1)) AS creator_did
		FROM "Tokens"
		WHERE token_type = 2
			AND split_part(token_id, '_', 1) ILIKE ?
		ORDER BY ft_name
		LIMIT ?
	`, prefix+"%", limit).Scan(&suggestions).Error
	return suggestions, err
}
