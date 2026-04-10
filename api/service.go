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

// dagEdgeRow is a shared type for recursive CTE results used by DAG functions.
type dagEdgeRow struct {
	ChildTxnID  string `gorm:"column:child_txn_id"`
	ParentTxnID string `gorm:"column:parent_txn_id"`
}

// GetDAGFromTxn returns the anchor transaction plus up to `depth` levels of ancestors,
// traversing the TokenChain backwards via previous_transaction_id.
// Nodes are all unique TransactionInfo records. Edges are directed child → parent links.
func GetDAGFromTxn(txnID string, depth int) (model.DAGResponse, error) {
	const maxPrevParents = 20

	if depth <= 0 || depth > 100 {
		depth = 100
	}

	var edges []dagEdgeRow
	if err := database.ReadDB.Raw(`
		WITH RECURSIVE dag_edges AS (

			-- Base case (limit parents)
			SELECT child_txn_id, parent_txn_id, depth FROM (
				SELECT
					transaction_id AS child_txn_id,
					previous_transaction_id AS parent_txn_id,
					1 AS depth,
					ROW_NUMBER() OVER (
						PARTITION BY transaction_id 
						ORDER BY transaction_id
					) as rn
				FROM "TokenChain"
				WHERE transaction_id = ?
				  AND previous_transaction_id IS NOT NULL
				  AND previous_transaction_id <> ''
			) t
			WHERE rn <= ?

			UNION

			-- Recursive step (limit parents per node)
			SELECT child_txn_id, parent_txn_id, depth FROM (
				SELECT
					tc.transaction_id AS child_txn_id,
					tc.previous_transaction_id AS parent_txn_id,
					de.depth + 1 AS depth,
					ROW_NUMBER() OVER (
						PARTITION BY tc.transaction_id 
						ORDER BY tc.transaction_id
					) as rn
				FROM "TokenChain" tc
				INNER JOIN dag_edges de 
					ON tc.transaction_id = de.parent_txn_id
				WHERE de.depth < ?
				  AND tc.previous_transaction_id IS NOT NULL
				  AND tc.previous_transaction_id <> ''
			) t
			WHERE rn <= ?
		)

		SELECT DISTINCT child_txn_id, parent_txn_id FROM dag_edges
	`, txnID, maxPrevParents, depth, maxPrevParents).Scan(&edges).Error; err != nil {
		return model.DAGResponse{}, err
	}

	// Collect unique txn IDs
	seen := map[string]struct{}{txnID: {}}
	for _, e := range edges {
		seen[e.ChildTxnID] = struct{}{}
		seen[e.ParentTxnID] = struct{}{}
	}

	return buildDAGResponse(edges), nil
}

// buildDAGResponse converts edge rows into DAGTxn nodes, grouping parents per
// child and capping at 20 unique parents per transaction.
func buildDAGResponse(edges []dagEdgeRow) model.DAGResponse {
	const maxParents = 6
	// parentMap: child -> ordered unique parents (max 20)
	parentMap := make(map[string][]string)
	parentSeen := make(map[string]map[string]struct{})

	for _, e := range edges {
		if _, ok := parentSeen[e.ChildTxnID]; !ok {
			parentSeen[e.ChildTxnID] = make(map[string]struct{})
		}
		if _, exists := parentSeen[e.ChildTxnID][e.ParentTxnID]; !exists {
			if len(parentMap[e.ChildTxnID]) < maxParents {
				parentMap[e.ChildTxnID] = append(parentMap[e.ChildTxnID], e.ParentTxnID)
				parentSeen[e.ChildTxnID][e.ParentTxnID] = struct{}{}
			}
		}
	}

	txns := make([]model.DAGTxn, 0, len(parentMap))
	for childID, parents := range parentMap {
		txns = append(txns, model.DAGTxn{
			TransactionID:          childID,
			PreviousTransactionIDs: parents,
		})
	}
	return model.DAGResponse{Transactions: txns}
}

// GetDAGTransactions fetches anchors ordered by epoch DESC (anchorBatch at a time),
// walks ancestors up to 5 levels deep (max 5 parents per node).
// offset controls which batch of anchors to start from for "show more" pagination.
// Falls back to TransactionInfo.tokens JSONB for any node TokenChain has no entry for.
func GetDAGTransactions(offset int) (model.DAGResponse, error) {
	const anchorBatch = 50
	const depth = 5
	const maxParents = 5

	// Step 1: fetch anchor batch ordered by epoch DESC with offset
	var anchors []struct {
		TransactionID string `gorm:"column:transaction_id"`
	}
	if err := database.ReadDB.Table("TransactionInfo").
		Select("transaction_id").
		Order("epoch DESC").
		Limit(anchorBatch).
		Offset(offset).
		Scan(&anchors).Error; err != nil {
		return model.DAGResponse{}, err
	}
	if len(anchors) == 0 {
		return model.DAGResponse{}, nil
	}
	anchorIDs := make([]string, len(anchors))
	for i, a := range anchors {
		anchorIDs[i] = a.TransactionID
	}

	// Step 2: recursive CTE — walk up to `depth` ancestor levels.
	// Parents are capped per node in Go (maxParents). DISTINCT prevents cycles.
	var edges []dagEdgeRow
	if err := database.ReadDB.Raw(`
		WITH RECURSIVE dag AS (
			SELECT DISTINCT
				transaction_id          AS child_txn_id,
				previous_transaction_id AS parent_txn_id,
				1                       AS depth
			FROM "TokenChain"
			WHERE transaction_id IN ?
			  AND previous_transaction_id IS NOT NULL
			  AND previous_transaction_id <> ''

			UNION

			SELECT DISTINCT
				tc.transaction_id          AS child_txn_id,
				tc.previous_transaction_id AS parent_txn_id,
				dag.depth + 1              AS depth
			FROM "TokenChain" tc
			JOIN dag ON tc.transaction_id = dag.parent_txn_id
			WHERE dag.depth < ?
			  AND tc.previous_transaction_id IS NOT NULL
			  AND tc.previous_transaction_id <> ''
		)
		SELECT DISTINCT child_txn_id, parent_txn_id FROM dag
	`, anchorIDs, depth).Scan(&edges).Error; err != nil {
		return model.DAGResponse{}, err
	}

	// Step 3: build ordered maps (anchors first, then discovered nodes in order)
	txnParents := make(map[string][]string)
	parentSeen := make(map[string]map[string]struct{})
	orderedIDs := make([]string, 0)
	seenIDs := make(map[string]struct{})

	addNode := func(id string) {
		if _, ok := seenIDs[id]; !ok {
			seenIDs[id] = struct{}{}
			txnParents[id] = []string{}
			parentSeen[id] = make(map[string]struct{})
			orderedIDs = append(orderedIDs, id)
		}
	}

	for _, id := range anchorIDs {
		addNode(id)
	}
	for _, e := range edges {
		addNode(e.ChildTxnID)
		addNode(e.ParentTxnID)
		ps := parentSeen[e.ChildTxnID]
		if _, exists := ps[e.ParentTxnID]; !exists && len(txnParents[e.ChildTxnID]) < maxParents {
			txnParents[e.ChildTxnID] = append(txnParents[e.ChildTxnID], e.ParentTxnID)
			ps[e.ParentTxnID] = struct{}{}
		}
	}

	// Step 4: JSONB fallback — batch-query TransactionInfo.tokens for any node
	// that TokenChain returned zero parents for.
	noParents := make([]string, 0)
	for _, id := range orderedIDs {
		if len(txnParents[id]) == 0 {
			noParents = append(noParents, id)
		}
	}
	if len(noParents) > 0 {
		var fallbackRows []dagEdgeRow
		if err := database.ReadDB.Raw(`
			SELECT DISTINCT
				transaction_id AS child_txn_id,
				elem->>'previousTransactionID' AS parent_txn_id
			FROM "TransactionInfo",
			     jsonb_array_elements(
			         COALESCE(tokens->'rbt', '[]'::jsonb) ||
			         COALESCE(tokens->'ft', '[]'::jsonb) ||
			         COALESCE(tokens->'nft', '[]'::jsonb) ||
			         COALESCE(tokens->'smartContract', '[]'::jsonb)
			     ) AS elem
			WHERE transaction_id IN ?
			  AND elem->>'previousTransactionID' IS NOT NULL
			  AND elem->>'previousTransactionID' <> ''
		`, noParents).Scan(&fallbackRows).Error; err == nil {
			for _, row := range fallbackRows {
				addNode(row.ChildTxnID)
				addNode(row.ParentTxnID)
				ps := parentSeen[row.ChildTxnID]
				if _, exists := ps[row.ParentTxnID]; !exists && len(txnParents[row.ChildTxnID]) < maxParents {
					txnParents[row.ChildTxnID] = append(txnParents[row.ChildTxnID], row.ParentTxnID)
					ps[row.ParentTxnID] = struct{}{}
				}
			}
		}
	}

	const maxTxns = 300
	if len(orderedIDs) > maxTxns {
		orderedIDs = orderedIDs[:maxTxns]
	}

	txns := make([]model.DAGTxn, 0, len(orderedIDs))
	for _, id := range orderedIDs {
		txns = append(txns, model.DAGTxn{
			TransactionID:          id,
			PreviousTransactionIDs: txnParents[id],
		})
	}
	return model.DAGResponse{Transactions: txns}, nil
}

// GetDAGWithSearch returns the normal DAG (same as GetDAGTransactions) merged with
// the searched txnID and its ancestors up to depth 5 (max 6 parents per level).
// The searched txn and its ancestors are prepended so they appear first.
func GetDAGWithSearch(searchTxnID string) (model.DAGResponse, error) {
	const depth = 5
	const maxParents = 5

	// --- Step 1: recursive CTE from searched txnID (same pattern as GetDAGTransactions) ---

	var edges []dagEdgeRow
	if err := database.ReadDB.Raw(`
		WITH RECURSIVE dag AS (
			SELECT DISTINCT
				transaction_id          AS child_txn_id,
				previous_transaction_id AS parent_txn_id,
				1                       AS depth
			FROM "TokenChain"
			WHERE transaction_id = ?
			  AND previous_transaction_id IS NOT NULL
			  AND previous_transaction_id <> ''

			UNION

			SELECT DISTINCT
				tc.transaction_id          AS child_txn_id,
				tc.previous_transaction_id AS parent_txn_id,
				dag.depth + 1              AS depth
			FROM "TokenChain" tc
			JOIN dag ON tc.transaction_id = dag.parent_txn_id
			WHERE dag.depth < ?
			  AND tc.previous_transaction_id IS NOT NULL
			  AND tc.previous_transaction_id <> ''
		)
		SELECT DISTINCT child_txn_id, parent_txn_id FROM dag
	`, searchTxnID, depth).Scan(&edges).Error; err != nil {
		return model.DAGResponse{}, err
	}

	// Build ordered map for the searched txn's subgraph
	searchParents := make(map[string][]string)
	parentSeen := make(map[string]map[string]struct{})
	searchOrdered := make([]string, 0)
	searchSeen := make(map[string]struct{})

	addSearchNode := func(id string) {
		if _, ok := searchSeen[id]; !ok {
			searchSeen[id] = struct{}{}
			searchParents[id] = []string{}
			parentSeen[id] = make(map[string]struct{})
			searchOrdered = append(searchOrdered, id)
		}
	}

	addSearchNode(searchTxnID)
	for _, e := range edges {
		addSearchNode(e.ChildTxnID)
		addSearchNode(e.ParentTxnID)
		ps := parentSeen[e.ChildTxnID]
		if _, exists := ps[e.ParentTxnID]; !exists && len(searchParents[e.ChildTxnID]) < maxParents {
			searchParents[e.ChildTxnID] = append(searchParents[e.ChildTxnID], e.ParentTxnID)
			ps[e.ParentTxnID] = struct{}{}
		}
	}

	// JSONB fallback for nodes with no TokenChain parents
	noParents := make([]string, 0)
	for _, id := range searchOrdered {
		if len(searchParents[id]) == 0 {
			noParents = append(noParents, id)
		}
	}
	if len(noParents) > 0 {
		var fallbackRows []dagEdgeRow
		if err := database.ReadDB.Raw(`
			SELECT DISTINCT
				transaction_id AS child_txn_id,
				elem->>'previousTransactionID' AS parent_txn_id
			FROM "TransactionInfo",
			     jsonb_array_elements(
			         COALESCE(tokens->'rbt', '[]'::jsonb) ||
			         COALESCE(tokens->'ft', '[]'::jsonb) ||
			         COALESCE(tokens->'nft', '[]'::jsonb) ||
			         COALESCE(tokens->'smartContract', '[]'::jsonb)
			     ) AS elem
			WHERE transaction_id IN ?
			  AND elem->>'previousTransactionID' IS NOT NULL
			  AND elem->>'previousTransactionID' <> ''
		`, noParents).Scan(&fallbackRows).Error; err == nil {
			for _, row := range fallbackRows {
				addSearchNode(row.ChildTxnID)
				addSearchNode(row.ParentTxnID)
				ps := parentSeen[row.ChildTxnID]
				if _, exists := ps[row.ParentTxnID]; !exists && len(searchParents[row.ChildTxnID]) < maxParents {
					searchParents[row.ChildTxnID] = append(searchParents[row.ChildTxnID], row.ParentTxnID)
					ps[row.ParentTxnID] = struct{}{}
				}
			}
		}
	}

	// --- Step 2: Get the normal DAG ---
	baseDAG, err := GetDAGTransactions(0)
	if err != nil {
		return model.DAGResponse{}, err
	}

	// --- Step 3: Merge — search txn + ancestors first, then base DAG deduplicated ---
	merged := make([]model.DAGTxn, 0, len(searchOrdered)+len(baseDAG.Transactions))
	seen := make(map[string]struct{}, len(searchOrdered)+len(baseDAG.Transactions))

	for _, txnID := range searchOrdered {
		merged = append(merged, model.DAGTxn{
			TransactionID:          txnID,
			PreviousTransactionIDs: searchParents[txnID],
		})
		seen[txnID] = struct{}{}
	}
	for _, t := range baseDAG.Transactions {
		if _, exists := seen[t.TransactionID]; !exists {
			merged = append(merged, t)
			seen[t.TransactionID] = struct{}{}
		}
	}

	return model.DAGResponse{Transactions: merged}, nil
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
