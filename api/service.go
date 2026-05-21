package api

import (
	"encoding/json"
	"explorer-server/database"
	"explorer-server/database/models"
	"explorer-server/model"
	"fmt"
	"net/http"
	"sort"
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
	// Fallback: Check FailedTransactionInfo
	if err := database.ReadDB.Table("FailedTransactionInfo").Where("transaction_id = ?", query).First(&txn).Error; err == nil {
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

type RBTSupplyStats struct {
	CirculatingSupply float64 `json:"circulating_supply"`
	TotalSupply       int64   `json:"total_supply"`
	FTCount           int64   `json:"ft_count"`
	NFTCount          int64   `json:"nft_count"`
	SCCount           int64   `json:"sc_count"`
	RBTPrice          float64 `json:"rbt_price"`
	TVL               float64 `json:"tvl"`
}

func fetchRBTPrice() (float64, error) {
	resp, err := http.Get("https://api.coingecko.com/api/v3/simple/price?ids=rubix&vs_currencies=usd")
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	// CoinGecko response: {"rubix":{"usd":98.84}}
	var body map[string]map[string]float64
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return 0, err
	}

	price, ok := body["rubix"]["usd"]
	if !ok {
		return 0, fmt.Errorf("rubix/usd price missing from CoinGecko response")
	}
	return price, nil
}

func GetRBTSupplyStats() (RBTSupplyStats, error) {
	var stats RBTSupplyStats

	// Circulating supply = sum of all RBT balances held by DIDs
	if err := database.ReadDB.Table("DIDBalances").
		Where("asset_type = ?", "RBT").
		Select("COALESCE(SUM(balance), 0)").
		Scan(&stats.CirculatingSupply).Error; err != nil {
		return stats, err
	}

	// Total supply = count of RBT mint transactions (RBT tokens with no previousTransactionID), each worth 1 RBT
	if err := database.ReadDB.Raw(`
		SELECT COUNT(*) FROM "Tokens"
		WHERE token_type = 1
		  AND token_id ~ '^[^_]+_[^_]+$'
	`).Scan(&stats.TotalSupply).Error; err != nil {
		return stats, err
	}

	// FT, NFT, SC counts from the Tokens table by token_type
	if err := database.ReadDB.Model(&models.Token{}).Where("token_type = ?", 2).Count(&stats.FTCount).Error; err != nil {
		return stats, err
	}
	if err := database.ReadDB.Model(&models.Token{}).Where("token_type = ?", 3).Count(&stats.NFTCount).Error; err != nil {
		return stats, err
	}
	if err := database.ReadDB.Model(&models.Token{}).Where("token_type = ?", 4).Count(&stats.SCCount).Error; err != nil {
		return stats, err
	}

	// Price from external API; TVL = (total_supply - circulating_supply) * price
	price, err := fetchRBTPrice()
	if err == nil {
		stats.RBTPrice = price
		stats.TVL = float64(stats.TotalSupply-int64(stats.CirculatingSupply)) * price
	}

	return stats, nil
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

// hasParent checks if a txn has any previous_transaction_id in TokenChain.
func hasParent(txnID string) bool {
	var count int64
	database.ReadDB.Raw(`
		SELECT COUNT(*) FROM "TokenChain"
		WHERE transaction_id = ?
		  AND previous_transaction_id IS NOT NULL
		  AND previous_transaction_id <> ''
		LIMIT 1
	`, txnID).Scan(&count)
	return count > 0
}

// chainScore recursively counts ancestor levels behind txnID, up to maxDepth.
func chainScore(txnID string, maxDepth int) int {
	if maxDepth == 0 {
		return 0
	}
	var row struct {
		ParentTxnID string `gorm:"column:parent_txn_id"`
	}
	database.ReadDB.Raw(`
		SELECT previous_transaction_id AS parent_txn_id
		FROM "TokenChain"
		WHERE transaction_id = ?
		  AND previous_transaction_id IS NOT NULL
		  AND previous_transaction_id <> ''
		LIMIT 1
	`, txnID).Scan(&row)
	if row.ParentTxnID == "" {
		return 0
	}
	return 1 + chainScore(row.ParentTxnID, maxDepth-1)
}

// fetchParents gets all distinct parent IDs for a txn from TokenChain,
// scores each by chain depth up to 5 levels, returns top maxParents sorted deepest-chain first.
func fetchParents(txnID string, maxParents int) []string {
	var rows []struct {
		ParentTxnID string `gorm:"column:parent_txn_id"`
	}
	database.ReadDB.Raw(`
		SELECT DISTINCT previous_transaction_id AS parent_txn_id
		FROM "TokenChain"
		WHERE transaction_id = ?
		  AND previous_transaction_id IS NOT NULL
		  AND previous_transaction_id <> ''
	`, txnID).Scan(&rows)

	candidates := make([]string, 0, len(rows))
	for _, r := range rows {
		if r.ParentTxnID != "" {
			candidates = append(candidates, r.ParentTxnID)
		}
	}
	if len(candidates) == 0 {
		return nil
	}

	type scored struct {
		id    string
		score int
	}
	list := make([]scored, len(candidates))
	for i, c := range candidates {
		list[i] = scored{id: c, score: chainScore(c, 5)}
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].score > list[j].score
	})

	result := make([]string, 0, maxParents)
	for _, s := range list {
		if len(result) >= maxParents {
			break
		}
		result = append(result, s.id)
	}
	return result
}

// walkTxn recursively saves parent IDs for txnID up to maxDepth levels.
// visited prevents processing the same txn twice across the whole DAG.
func walkTxn(txnID string, level int, maxDepth int, maxParents int,
	txnParents map[string][]string, orderedIDs *[]string, visited map[string]struct{}) {

	if level >= maxDepth {
		return
	}
	if _, done := visited[txnID]; done {
		return
	}
	visited[txnID] = struct{}{}

	parents := fetchParents(txnID, maxParents)
	for _, p := range parents {
		txnParents[txnID] = append(txnParents[txnID], p)
		// Register parent node if new
		if _, exists := txnParents[p]; !exists {
			txnParents[p] = []string{}
			*orderedIDs = append(*orderedIDs, p)
		}
		walkTxn(p, level+1, maxDepth, maxParents, txnParents, orderedIDs, visited)
	}
}

// GetDAGTransactions fetches the latest `anchorBatch` txns by epoch DESC (with offset),
// then for each txn recursively walks parents up to 5 levels (max 5 parents per txn).
func GetDAGTransactions(offset int) (model.DAGResponse, error) {
	const anchorBatch = 60
	const depth = 5
	const maxParents = 7
	const maxTxns = 500

	// Step 1: fetch latest txns as primary nodes
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

	txnParents := make(map[string][]string)
	orderedIDs := make([]string, 0)
	visited := make(map[string]struct{})

	// Register all primary txns first (preserves epoch DESC order)
	for _, a := range anchors {
		if _, exists := txnParents[a.TransactionID]; !exists {
			txnParents[a.TransactionID] = []string{}
			orderedIDs = append(orderedIDs, a.TransactionID)
		}
	}

	// Step 2: for each primary txn, recursively walk its parent chain
	for _, a := range anchors {
		walkTxn(a.TransactionID, 0, depth, maxParents, txnParents, &orderedIDs, visited)
	}

	// Step 3: build response (cap at maxTxns)
	txns := make([]model.DAGTxn, 0, len(orderedIDs))
	for _, id := range orderedIDs {
		txns = append(txns, model.DAGTxn{
			TransactionID:          id,
			PreviousTransactionIDs: txnParents[id],
		})
		if len(txns) >= maxTxns {
			break
		}
	}
	return model.DAGResponse{Transactions: txns}, nil
}

// GetDAGWithSearch prepends the searched txn (and its parent chain up to 5 levels)
// to the normal DAG response. Use offset for pagination of the base DAG.
func GetDAGWithSearch(searchTxnID string, offset int) (model.DAGResponse, error) {
	const depth = 5
	const maxParents = 7

	// Step 1: walk the searched txn's parent chain
	txnParents := make(map[string][]string)
	orderedIDs := make([]string, 0)
	visited := make(map[string]struct{})

	txnParents[searchTxnID] = []string{}
	orderedIDs = append(orderedIDs, searchTxnID)

	walkTxn(searchTxnID, 0, depth, maxParents, txnParents, &orderedIDs, visited)

	// Step 2: get normal DAG
	baseDAG, err := GetDAGTransactions(offset)
	if err != nil {
		return model.DAGResponse{}, err
	}

	// Step 3: merge — searched txn + its ancestors first, then base DAG (deduped)
	seen := make(map[string]struct{}, len(orderedIDs)+len(baseDAG.Transactions))
	merged := make([]model.DAGTxn, 0, len(orderedIDs)+len(baseDAG.Transactions))

	for _, id := range orderedIDs {
		merged = append(merged, model.DAGTxn{
			TransactionID:          id,
			PreviousTransactionIDs: txnParents[id],
		})
		seen[id] = struct{}{}
	}
	for _, t := range baseDAG.Transactions {
		if _, exists := seen[t.TransactionID]; !exists {
			merged = append(merged, t)
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

	// 1. Calculate total count
	var total int64
	countQuery := `
		SELECT SUM(c) FROM (
			SELECT COUNT(*) as c FROM "TransactionInfo" WHERE amount > 0
			UNION ALL
			SELECT COUNT(*) as c FROM "FailedTransactionInfo" WHERE amount > 0
		) AS combined
	`
	if err := database.ReadDB.Raw(countQuery).Scan(&total).Error; err != nil {
		return nil, 0, err
	}

	// 2. Fetch paginated results
	// Optimization: Sort individual tables before union to allow Postgres to use indexes.
	// This reduces the 'Slow SQL' overhead significantly on large datasets.
	var result []models.TransactionInfo
	dataQuery := `
		SELECT transaction_id, initiator,
			COALESCE(NULLIF(owner, ''), tokens->'rbt'->0->>'tokenId', tokens->'ft'->0->>'tokenId', tokens->'nft'->0->>'tokenId', tokens->'smartContract'->0->>'tokenId') AS owner,
			epoch, network, tokens, committed_tokens, quorums, memo, status, amount, created_at, updated_at
		FROM (
			(SELECT transaction_id, initiator, owner, tokens, epoch, network, committed_tokens, quorums, memo, status, amount, created_at, updated_at FROM "TransactionInfo" WHERE amount > 0 ORDER BY created_at DESC LIMIT ?)
			UNION ALL
			(SELECT transaction_id, initiator, owner, tokens, epoch, network, committed_tokens, quorums, memo, status, amount, created_at, updated_at FROM "FailedTransactionInfo" WHERE amount > 0 ORDER BY created_at DESC LIMIT ?)
		) AS combined
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`
	// We use limit + offset for the subqueries to ensure the global top N is captured.
	subLimit := limit + offset
	if err := database.ReadDB.Raw(dataQuery, subLimit, subLimit, limit, offset).Scan(&result).Error; err != nil {
		return nil, 0, err
	}

	if result == nil {
		result = make([]models.TransactionInfo, 0)
	}

	return result, total, nil
}

// GetTransactionSummaryList returns a lightweight list of transactions (no token details)
func GetTransactionSummaryList(limit, page int) ([]models.TransactionSummary, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 10
	}
	offset := (page - 1) * limit

	// 1. Calculate total count
	var total int64
	countQuery := `
		SELECT SUM(c) FROM (
			SELECT COUNT(*) as c FROM "TransactionInfo" WHERE amount > 0
			UNION ALL
			SELECT COUNT(*) as c FROM "FailedTransactionInfo" WHERE amount > 0
		) AS combined
	`
	if err := database.ReadDB.Raw(countQuery).Scan(&total).Error; err != nil {
		return nil, 0, err
	}

	// 2. Fetch paginated results (Summary ONLY)
	var result []models.TransactionSummary
	dataQuery := `
		SELECT transaction_id, initiator,
			COALESCE(NULLIF(owner, ''), tokens->'rbt'->0->>'tokenId', tokens->'ft'->0->>'tokenId', tokens->'nft'->0->>'tokenId', tokens->'smartContract'->0->>'tokenId') AS owner,
			epoch, network, status, amount, created_at
		FROM (
			(SELECT transaction_id, initiator, owner, tokens, epoch, network, status, amount, created_at FROM "TransactionInfo" WHERE amount > 0 ORDER BY created_at DESC LIMIT ?)
			UNION ALL
			(SELECT transaction_id, initiator, owner, tokens, epoch, network, status, amount, created_at FROM "FailedTransactionInfo" WHERE amount > 0 ORDER BY created_at DESC LIMIT ?)
		) AS combined
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`
	subLimit := limit + offset
	if err := database.ReadDB.Raw(dataQuery, subLimit, subLimit, limit, offset).Scan(&result).Error; err != nil {
		return nil, 0, err
	}

	if result == nil {
		result = make([]models.TransactionSummary, 0)
	}

	return result, total, nil
}

// GetCurrentlyPledgedTransactionsList returns transactions that currently have pledged tokens
func GetCurrentlyPledgedTransactionsList(limit, page int) ([]models.TransactionSummary, int64, float64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 10
	}
	offset := (page - 1) * limit

	var counts struct {
		TxCount    int64
		TokenValue float64
	}
	countQuery := `
		SELECT COUNT(DISTINCT transaction_id) as tx_count, COALESCE(SUM(token_value), 0) as token_value 
		FROM "Tokens" 
		WHERE token_status IN (6, 7)
	`
	if err := database.ReadDB.Raw(countQuery).Scan(&counts).Error; err != nil {
		return nil, 0, 0, err
	}

	var result []models.TransactionSummary
	dataQuery := `
		SELECT transaction_id, initiator,
			COALESCE(NULLIF(owner, ''), tokens->'rbt'->0->>'tokenId', tokens->'ft'->0->>'tokenId', tokens->'nft'->0->>'tokenId', tokens->'smartContract'->0->>'tokenId') AS owner,
			epoch, network, status, amount, created_at
		FROM (
			(
				SELECT transaction_id, initiator, owner, tokens, epoch, network, status, amount, created_at 
				FROM "TransactionInfo" ti
				WHERE EXISTS (SELECT 1 FROM "Tokens" t WHERE t.transaction_id = ti.transaction_id AND t.token_status IN (6, 7))
				ORDER BY epoch DESC 
				LIMIT ?
			)
			UNION ALL
			(
				SELECT transaction_id, initiator, owner, tokens, epoch, network, status, amount, created_at 
				FROM "FailedTransactionInfo" ti
				WHERE EXISTS (SELECT 1 FROM "Tokens" t WHERE t.transaction_id = ti.transaction_id AND t.token_status IN (6, 7))
				ORDER BY epoch DESC 
				LIMIT ?
			)
		) AS combined
		ORDER BY epoch DESC
		LIMIT ? OFFSET ?
	`
	subLimit := limit + offset
	if err := database.ReadDB.Raw(dataQuery, subLimit, subLimit, limit, offset).Scan(&result).Error; err != nil {
		return nil, 0, 0, err
	}

	if result == nil {
		result = make([]models.TransactionSummary, 0)
	}

	return result, counts.TxCount, counts.TokenValue, nil
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
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 10
	}
	offset := (page - 1) * limit

	// FT token_id format: <ftName>_<creatorDID>_<index> — extract directly from Tokens.
	var total int64
	if err := database.ReadDB.Raw(`
		SELECT COUNT(*) FROM (
			SELECT DISTINCT
				split_part(token_id, '_', 1) AS ft_name,
				split_part(token_id, '_', 2) AS creator_did
			FROM "Tokens"
			WHERE token_type = 2
		) AS distinct_groups
	`).Scan(&total).Error; err != nil {
		return nil, 0, err
	}

	var groups []model.FTGroup
	if err := database.ReadDB.Raw(`
		SELECT
			split_part(token_id, '_', 1) AS ft_name,
			split_part(token_id, '_', 2) AS creator_did,
			COUNT(*) AS count,
			MAX(token_value) AS ft_value
		FROM "Tokens"
		WHERE token_type = 2
		GROUP BY ft_name, creator_did
		ORDER BY count DESC
		LIMIT ? OFFSET ?
	`, limit, offset).Scan(&groups).Error; err != nil {
		return nil, 0, err
	}

	if groups == nil {
		groups = make([]model.FTGroup, 0)
	}

	return groups, total, nil
}

func GetFTListByFTName(ftName string, creatorDID string, limit, page int) ([]models.Token, int64, error) {
	offset := (page - 1) * limit
	// FT token_id format: <ftName>_<creatorDID>_<index> — creatorDID is the middle bafy segment.
	base := database.ReadDB.Model(&models.Token{}).Where("token_type = ?", 2).Where("token_id LIKE ?", ftName+"_%")
	if creatorDID != "" {
		base = base.Where("token_id LIKE ?", "%_"+creatorDID+"_%")
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

// GetFTHoldersList returns DIDs ranked by total FT count, with a per-FT breakdown for each DID.
// FT token_id format: <ftName>_<creatorDID>_<index>
func GetFTHoldersList(limit, page int) ([]model.FTHolderInfo, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 10
	}
	offset := (page - 1) * limit

	// 1. Total number of distinct DIDs holding any FT
	var total int64
	if err := database.ReadDB.Raw(`
		SELECT COUNT(DISTINCT did) FROM "Tokens"
		WHERE token_type = 2 AND did != '' AND did != '0'
	`).Scan(&total).Error; err != nil {
		return nil, 0, err
	}

	// 2. Top DIDs by total FT count (paginated)
	type didTotalRow struct {
		DID          string `gorm:"column:did"`
		TotalFTCount int64  `gorm:"column:total_ft_count"`
	}
	var didTotals []didTotalRow
	if err := database.ReadDB.Raw(`
		SELECT did, COUNT(*) AS total_ft_count
		FROM "Tokens"
		WHERE token_type = 2 AND did != '' AND did != '0'
		GROUP BY did
		ORDER BY total_ft_count DESC
		LIMIT ? OFFSET ?
	`, limit, offset).Scan(&didTotals).Error; err != nil {
		return nil, 0, err
	}

	if len(didTotals) == 0 {
		return []model.FTHolderInfo{}, total, nil
	}

	// 3. Per-FT breakdown for those DIDs only
	dids := make([]string, len(didTotals))
	for i, dt := range didTotals {
		dids[i] = dt.DID
	}

	type breakdownRow struct {
		DID        string  `gorm:"column:did"`
		FTName     string  `gorm:"column:ft_name"`
		CreatorDID string  `gorm:"column:creator_did"`
		Count      float64 `gorm:"column:count"`
	}
	var breakdowns []breakdownRow
	if err := database.ReadDB.Raw(`
		SELECT
			did,
			split_part(token_id, '_', 1) AS ft_name,
			split_part(token_id, '_', 2) AS creator_did,
			COUNT(*) AS count
		FROM "Tokens"
		WHERE token_type = 2 AND did IN ?
		GROUP BY did, ft_name, creator_did
		ORDER BY count DESC
	`, dids).Scan(&breakdowns).Error; err != nil {
		return nil, 0, err
	}

	// 4. Merge breakdowns under each DID
	holdingsByDID := make(map[string][]model.FTHolding)
	for _, b := range breakdowns {
		holdingsByDID[b.DID] = append(holdingsByDID[b.DID], model.FTHolding{
			FTName:     b.FTName,
			CreatorDID: b.CreatorDID,
			Count:      b.Count,
		})
	}

	result := make([]model.FTHolderInfo, len(didTotals))
	for i, dt := range didTotals {
		holdings := holdingsByDID[dt.DID]
		if holdings == nil {
			holdings = []model.FTHolding{}
		}
		result[i] = model.FTHolderInfo{
			DID:          dt.DID,
			TotalFTCount: dt.TotalFTCount,
			Holdings:     holdings,
		}
	}

	return result, total, nil
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
		// Fallback: Check FailedTransactionInfo
		if ferr := database.ReadDB.Table("FailedTransactionInfo").Where("transaction_id = ?", txnID).First(&transaction).Error; ferr != nil {
			return transaction, err // Return the original Error if both fail
		}
	}
	return transaction, nil
}

func GetTxnsByDID(did string, page, limit int) ([]models.TransactionSummary, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 10
	}
	offset := (page - 1) * limit

	// 1. Calculate combined total count
	var total int64
	countQuery := `
		SELECT COUNT(*) FROM (
			SELECT transaction_id FROM "TransactionInfo" WHERE initiator = ? OR owner = ?
			UNION ALL
			SELECT transaction_id FROM "FailedTransactionInfo" WHERE initiator = ? OR owner = ?
		) AS combined
	`
	if err := database.ReadDB.Raw(countQuery, did, did, did, did).Scan(&total).Error; err != nil {
		return nil, 0, err
	}

	// 2. Fetch combined paginated results (Summary ONLY)
	var transactions []models.TransactionSummary
	dataQuery := `
		SELECT * FROM (
			SELECT transaction_id, initiator, owner, epoch, network, status, amount, created_at FROM "TransactionInfo" WHERE initiator = ? OR owner = ?
			UNION ALL
			SELECT transaction_id, initiator, owner, epoch, network, status, amount, created_at FROM "FailedTransactionInfo" WHERE initiator = ? OR owner = ?
		) AS combined
		ORDER BY epoch DESC
		LIMIT ? OFFSET ?
	`
	if err := database.ReadDB.Raw(dataQuery, did, did, did, did, limit, offset).Scan(&transactions).Error; err != nil {
		return nil, 0, err
	}

	if transactions == nil {
		transactions = make([]models.TransactionSummary, 0)
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
// FT token_id format: <ftName>_<creatorDID>_<index>
func GetFTInfo(ftName, creatorDID string) (model.FTInfo, error) {
	var info model.FTInfo
	err := database.ReadDB.Raw(`
		SELECT
			split_part(token_id, '_', 1)                              AS ft_name,
			(regexp_match(token_id, 'bafy[a-zA-Z0-9]{55}'))[1]        AS creator_did,
			MAX(token_value)                                           AS ft_value,
			COUNT(*)                                                   AS total_amount,
			EXTRACT(EPOCH FROM MIN(created_at))::bigint                AS created_time
		FROM "Tokens"
		WHERE token_type = 2
			AND split_part(token_id, '_', 1) = ?
			AND (regexp_match(token_id, 'bafy[a-zA-Z0-9]{55}'))[1] = ?
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
		SELECT did, balance AS token_count
		FROM "DIDBalances"
		WHERE asset_type = 'FT' AND token_name = ? AND creator_did = ? AND balance > 0
		ORDER BY balance DESC
		LIMIT ? OFFSET ?
	`, ftName, creatorDID, limit, offset).Scan(&holders).Error
	if err != nil {
		return model.FTTopHoldersResponse{}, err
	}

	var countResult struct{ Count int64 }
	if err := database.ReadDB.Raw(`
		SELECT COUNT(*) AS count
		FROM "DIDBalances"
		WHERE asset_type = 'FT' AND token_name = ? AND creator_did = ? AND balance > 0
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
		SELECT DISTINCT token_name AS ft_name, creator_did
		FROM "DIDBalances"
		WHERE asset_type = 'FT'
			AND token_name ILIKE ?
			AND balance > 0
		ORDER BY ft_name
		LIMIT ?
	`, prefix+"%", limit).Scan(&suggestions).Error
	return suggestions, err
}
