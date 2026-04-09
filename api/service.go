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
	Data any `json:"data"`
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

	// Build DAGTxn pairs from edges
	txns := make([]model.DAGTxn, len(edges))
	for i, e := range edges {
		txns[i] = model.DAGTxn{TransactionID: e.ChildTxnID, PreviousTransactionID: e.ParentTxnID}
	}
	return model.DAGResponse{Transactions: txns, HasMore: false}, nil
}

// GetDAGTransactions fetches anchor txns in batches of 50, starting at `offset`.
// For each batch:
//   - All 50 anchors are added as nodes (with empty previous_transaction_id if no chain found).
//   - BFS walks up to depth 5, capping unique parents at 20 per level.
//   - Parent txns discovered by BFS are also added as nodes.
//
// Keeps fetching more batches until 500 unique txns are collected or no more exist.
// Returns next_offset for the Load More button.
func GetDAGTransactions(offset int) (model.DAGResponse, error) {
	const anchorBatch = 50
	const depth = 5
	const levelCap = 20
	const targetTxns = 500

	if offset < 0 {
		offset = 0
	}

	type edgeRow struct {
		ChildTxnID  string `gorm:"column:child_txn_id"`
		ParentTxnID string `gorm:"column:parent_txn_id"`
	}

	// txnMap: txn_id -> previous_txn_id ("" if genesis/unknown)
	// Using a map ensures every txn appears exactly once.
	txnMap := make(map[string]string, targetTxns)

	currentOffset := offset
	hasMore := false

	for len(txnMap) < targetTxns {
		// Fetch next batch of anchor IDs (one extra to detect has_more)
		var anchors []struct {
			TransactionID string `gorm:"column:transaction_id"`
		}
		if err := database.ReadDB.Table("TransactionInfo").
			Select("transaction_id").
			Order("epoch DESC").
			Limit(anchorBatch+1).
			Offset(currentOffset).
			Scan(&anchors).Error; err != nil {
			return model.DAGResponse{}, err
		}

		hasMore = len(anchors) > anchorBatch
		if hasMore {
			anchors = anchors[:anchorBatch]
		}
		if len(anchors) == 0 {
			hasMore = false
			break
		}
		currentOffset += len(anchors)

		// Add all anchors with empty previous — BFS will fill them in if chain exists
		frontier := make([]string, 0, len(anchors))
		for _, a := range anchors {
			if _, seen := txnMap[a.TransactionID]; !seen {
				txnMap[a.TransactionID] = ""
				frontier = append(frontier, a.TransactionID)
			}
		}

		// BFS: walk ancestors level by level
		for d := 0; d < depth && len(frontier) > 0; d++ {
			var rows []edgeRow
			if err := database.ReadDB.Raw(`
				SELECT DISTINCT transaction_id AS child_txn_id, previous_transaction_id AS parent_txn_id
				FROM "TokenChain"
				WHERE transaction_id IN ?
				  AND previous_transaction_id IS NOT NULL
				  AND previous_transaction_id <> ''
			`, frontier).Scan(&rows).Error; err != nil {
				return model.DAGResponse{}, err
			}
			if len(rows) == 0 {
				break
			}

			// Collect unique parents, capped at levelCap
			parentSeen := make(map[string]struct{}, levelCap)
			for _, row := range rows {
				if _, ok := parentSeen[row.ParentTxnID]; !ok {
					if len(parentSeen) >= levelCap {
						break
					}
					parentSeen[row.ParentTxnID] = struct{}{}
				}
			}

			// Update child->parent mapping; add new parent nodes
			for _, row := range rows {
				if _, parentOk := parentSeen[row.ParentTxnID]; !parentOk {
					continue
				}
				// Fill in previous_transaction_id for this child if not set yet
				if prev, exists := txnMap[row.ChildTxnID]; exists && prev == "" {
					txnMap[row.ChildTxnID] = row.ParentTxnID
				}
				// Add parent as a new node if not seen
				if _, seen := txnMap[row.ParentTxnID]; !seen {
					txnMap[row.ParentTxnID] = ""
				}
			}

			nextFrontier := make([]string, 0, len(parentSeen))
			for pid := range parentSeen {
				nextFrontier = append(nextFrontier, pid)
			}
			frontier = nextFrontier
		}
	}

	// Build output slice from map
	allTxns := make([]model.DAGTxn, 0, len(txnMap))
	for txnID, prevID := range txnMap {
		allTxns = append(allTxns, model.DAGTxn{
			TransactionID:         txnID,
			PreviousTransactionID: prevID,
		})
	}

	return model.DAGResponse{
		Transactions: allTxns,
		HasMore:      hasMore,
		NextOffset:   currentOffset,
	}, nil
}

func GetTransactionInfoList(limit, page int) ([]models.TransactionInfo, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 10
	}
	offset := (page - 1) * limit

	// 1. Calculate total count (where amount > 0 across both success and failure tables)
	var total int64
	countQuery := `
		SELECT COUNT(*) FROM (
			SELECT transaction_id FROM "TransactionInfo" WHERE amount > 0
			UNION ALL
			SELECT transaction_id FROM "FailedTransactionInfo" WHERE amount > 0
		) AS combined
	`
	if err := database.ReadDB.Raw(countQuery).Scan(&total).Error; err != nil {
		return nil, 0, err
	}

	// 2. Fetch paginated results
	var result []models.TransactionInfo
	dataQuery := `
		SELECT * FROM (
			SELECT transaction_id, initiator, owner, epoch, network, tokens, committed_tokens, quorums, memo, status, amount, created_at, updated_at FROM "TransactionInfo" WHERE amount > 0
			UNION ALL
			SELECT transaction_id, initiator, owner, epoch, network, tokens, committed_tokens, quorums, memo, status, amount, created_at, updated_at FROM "FailedTransactionInfo" WHERE amount > 0
		) AS combined
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`
	if err := database.ReadDB.Raw(dataQuery, limit, offset).Scan(&result).Error; err != nil {
		return nil, 0, err
	}

	if result == nil {
		result = make([]models.TransactionInfo, 0)
	}

	return result, total, nil
}

func GetDIDHoldersList(limit, page int) ([]models.DIDBalance, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 10
	}
	offset := (page - 1) * limit

	var total int64
	if err := database.ReadDB.Table("DIDBalances").Where("asset_type = ? AND balance > 0", "RBT").Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var balances []models.DIDBalance
	if err := database.ReadDB.Table("DIDBalances").Where("asset_type = ? AND balance > 0", "RBT").Order("balance DESC").Limit(limit).Offset(offset).Find(&balances).Error; err != nil {
		return nil, 0, err
	}
	return balances, total, nil
}

func GetDIDBalance(did string) ([]models.DIDBalance, error) {
	var balances []models.DIDBalance
	if err := database.ReadDB.Table("DIDBalances").Where("did = ?", did).Find(&balances).Error; err != nil {
		return nil, err
	}
	return balances, nil
}

func GetRBTList(limit, page int) ([]models.Token, int64, error) {
	offset := (page - 1) * limit

	var total int64
	if err := database.ReadDB.Model(&models.Token{}).Where("token_type = ?", 1).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var tokens []models.Token
	if err := database.ReadDB.Model(&models.Token{}).Where("token_type = ?", 1).Order("updated_at DESC").Limit(limit).Offset(offset).Find(&tokens).Error; err != nil {
		return nil, 0, err
	}
	return tokens, total, nil
}

func GetFTGroupList(limit, page int) ([]model.FTGroup, int64, error) {
	var tokens []models.Token
	if err := database.ReadDB.Model(&models.Token{}).Where("token_type = ?", 2).Find(&tokens).Error; err != nil {
		return nil, 0, err
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

	total := int64(len(allGroups))
	start := (page - 1) * limit
	if start > len(allGroups) {
		start = len(allGroups)
	}
	end := start + limit
	if end > len(allGroups) {
		end = len(allGroups)
	}
	return allGroups[start:end], total, nil
}

func GetFTListByFTName(ftName string, creatorDID string, limit, page int) ([]models.Token, int64, error) {
	offset := (page - 1) * limit
	base := database.ReadDB.Model(&models.Token{}).Where("token_type = ?", 2).Where("token_id LIKE ?", ftName+"_%")
	if creatorDID != "" {
		base = base.Where("token_id LIKE ?", "%_"+creatorDID)
	}

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var tokens []models.Token
	if err := base.Order("updated_at DESC").Limit(limit).Offset(offset).Find(&tokens).Error; err != nil {
		return nil, 0, err
	}
	return tokens, total, nil
}

func GetSCList(limit, page int) ([]models.Token, int64, error) {
	offset := (page - 1) * limit

	var total int64
	if err := database.ReadDB.Model(&models.Token{}).Where("token_type = ?", 4).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var tokens []models.Token
	if err := database.ReadDB.Model(&models.Token{}).Where("token_type = ?", 4).Order("updated_at DESC").Limit(limit).Offset(offset).Find(&tokens).Error; err != nil {
		return nil, 0, err
	}
	return tokens, total, nil
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

func GetTxnsByDID(did string, page, limit int) ([]models.TransactionInfo, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 10
	}
	offset := (page - 1) * limit

	var total int64
	if err := database.ReadDB.Table("TransactionInfo").
		Where("initiator = ? OR owner = ?", did, did).
		Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var transactions []models.TransactionInfo
	if err := database.ReadDB.Table("TransactionInfo").
		Where("initiator = ? OR owner = ?", did, did).
		Order("epoch DESC").
		Limit(limit).Offset(offset).
		Find(&transactions).Error; err != nil {
		return nil, 0, err
	}
	return transactions, total, nil
}

func GetTransactionInfoListByToken(tokenID string, page, limit int) ([]models.TransactionInfo, int64, error) {
	ids, total, err := GetTransactionIDList(tokenID, page, limit)
	if err != nil {
		return nil, 0, err
	}
	if len(ids) == 0 {
		return []models.TransactionInfo{}, total, nil
	}

	txnIDs := make([]string, len(ids))
	for i, entry := range ids {
		txnIDs[i] = entry.TransactionID
	}

	var transactions []models.TransactionInfo
	if err := database.ReadDB.Table("TransactionInfo").Where("transaction_id IN ?", txnIDs).Order("epoch DESC").Find(&transactions).Error; err != nil {
		return nil, 0, err
	}
	return transactions, total, nil
}

func GetTransactionIDList(tokenID string, page, limit int) ([]models.TokenChain, int64, error) {
	var tca models.TokenChainArray
	if err := database.ReadDB.Table("TokenChainArray").Where("token_id = ?", tokenID).First(&tca).Error; err != nil {
		return nil, 0, err
	}

	var chain []uint64
	if err := json.Unmarshal(tca.Index, &chain); err != nil {
		return nil, 0, err
	}

	total := int64(len(chain))
	if total == 0 {
		return []models.TokenChain{}, 0, nil
	}

	end := int(total) - ((page - 1) * limit)
	start := int(total) - (page * limit)
	if end <= 0 {
		return []models.TokenChain{}, total, nil
	}
	if start < 0 {
		start = 0
	}
	pagedIndices := chain[start:end]

	var history []models.TokenChain
	if err := database.ReadDB.Table("TokenChain").Where("id IN ?", pagedIndices).Order("id DESC").Find(&history).Error; err != nil {
		return nil, 0, err
	}
	return history, total, nil
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
