package constants

// ============================================================
//  A) ASSET TYPE ENUM  (IncomingBlockInfo.asset_type)
// ------------------------------------------------------------
//  These values come from fullnode asset classification.
//  Used in: IncomingBlockInfo.AssetType, DB updates, sync logic.
// ============================================================

const (
	RBTTokenAssetType           int = iota // 0
	SmartContractTokenAssetType            // 1
	NFTTokenAssetType                      // 2
	FTTokenAssetType                       // 3
)

// ============================================================
//  B) TTTokenTypeKey ENUM (Inside TITokensKey in block JSON)
// ------------------------------------------------------------
//  These values appear inside:
//    block["TCTransInfoKey"]["TITokensKey"][tokenID]["TTTokenTypeKey"]
//
//  DO NOT MIX WITH AssetType above — both enums must coexist.
// ============================================================

const (
	TT_RBTTokenType           = 0
	TT_PartTokenType          = 1
	TT_NFTTokenType           = 2
	TT_TestTokenType          = 3
	TT_TestPartTokenType      = 5
	TT_TestNFTTokenType       = 6
	TT_SmartContractTokenType = 8
	TT_TestSmartContractType  = 9
	TT_FTTokenType            = 10
)

// ============================================================
//  C) Transaction Types (TCTransTypeKey)
// ------------------------------------------------------------
//  These values appear in TCTransTypeKey ("01", "02", ...).
// ============================================================

const (
	TokenMintedType       = "01"
	TokenTransferredType  = "02"
	TokenMigratedType     = "03"
	TokenPledgedType      = "04"
	TokenGeneratedType    = "05"
	TokenUnpledgedType    = "06"
	TokenCommittedType    = "07"
	TokenBurntType        = "08"
	TokenDeployedType     = "09"
	TokenExecutedType     = "10"
	TokenContractCommited = "11"
	TokenPinnedAsService  = "12"
	TokenIsBurntForFT     = "13"
)

// ============================================================
//  HELPER FUNCTIONS
// ============================================================

// ------------------------------------------------------------
// Converts ASSET TYPE (0–3) → readable string
// ------------------------------------------------------------
func AssetTypeToString(assetType int) string {
	switch assetType {
	case RBTTokenAssetType:
		return "RBT"
	case SmartContractTokenAssetType:
		return "SmartContract"
	case NFTTokenAssetType:
		return "NFT"
	case FTTokenAssetType:
		return "FT"
	default:
		return "Unknown"
	}
}

// ------------------------------------------------------------
// Converts TTTokenTypeKey → normalized ASSET TYPE
//
// WARNING:
//   This mapping ONLY applies to cases where we must classify
//   a token inside TITokensKey for explorer token-table updates.
// ------------------------------------------------------------
func MapTTTypeToAssetType(tt int) int {
	switch tt {

	case TT_RBTTokenType:
		return RBTTokenAssetType

	case TT_NFTTokenType, TT_TestNFTTokenType:
		return NFTTokenAssetType

	case TT_SmartContractTokenType, TT_TestSmartContractType:
		return SmartContractTokenAssetType

	case TT_FTTokenType, TT_TestTokenType:
		return FTTokenAssetType

	// Ignore test/part tokens unless needed later
	case TT_PartTokenType, TT_TestPartTokenType:
		return RBTTokenAssetType // part tokens behave like RBT in explorer

	default:
		return RBTTokenAssetType
	}
}

// ------------------------------------------------------------
// Converts TCTransTypeKey to human-readable string
// ------------------------------------------------------------
func TxTypeToString(txType string) string {
	switch txType {
	case TokenMintedType:
		return "Minted"
	case TokenTransferredType:
		return "Transferred"
	case TokenMigratedType:
		return "Migrated"
	case TokenPledgedType:
		return "Pledged"
	case TokenGeneratedType:
		return "Generated"
	case TokenUnpledgedType:
		return "Unpledged"
	case TokenCommittedType:
		return "Committed"
	case TokenBurntType:
		return "Burnt"
	case TokenDeployedType:
		return "Deployed"
	case TokenExecutedType:
		return "Executed"
	case TokenContractCommited:
		return "ContractCommitted"
	case TokenPinnedAsService:
		return "PinnedAsService"
	case TokenIsBurntForFT:
		return "BurntForFT"
	default:
		return "Unknown"
	}
}
