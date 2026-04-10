package models

// PubSub Topics
const (
	Event_RubixTxns = "rubix_txn"
	Event_RubixDID  = "rubix_did"
)

// Token Roles (Aligned with Rubix Core: index + 1)
const (
	TokenRole_Mint     int16 = 1
	TokenRole_Transfer int16 = 2
	TokenRole_Execute  int16 = 3
	TokenRole_Deploy   int16 = 4
	TokenRole_Burn     int16 = 5
	TokenRole_Commit   int16 = 6
	TokenRole_Uncommit int16 = 7
	TokenRole_Pledge   int16 = 8
	TokenRole_Unpledge int16 = 9
)

// Token Status (Aligned with Rubix Core)
const (
	TokenStatus_Free             int16 = 0
	TokenStatus_Locked           int16 = 1
	TokenStatus_Generated        int16 = 2
	TokenStatus_Fetched          int16 = 3
	TokenStatus_Transferred      int16 = 4
	TokenStatus_Committed        int16 = 5
	TokenStatus_Pledged          int16 = 6
	TokenStatus_QuorumPledged    int16 = 7
	TokenStatus_Burnt            int16 = 8
	TokenStatus_BurntForFT       int16 = 9
	TokenStatus_Deployed         int16 = 10
	TokenStatus_Executed         int16 = 11
	TokenStatus_PinnedAsService  int16 = 12
	TokenStatus_Orphaned         int16 = 13
	TokenStatus_ChainSyncIssue   int16 = 14
	TokenStatus_BeingDoubleSpent int16 = 15
	TokenStatus_Unpledged        int16 = 16
)

// TokenRole represents a role definition
type TokenRole struct {
	Name     string
	IsActive bool
}

// TokenRoleTypes defines the mapping for lookup (index + 1 = RoleID)
var TokenRoleTypes = []TokenRole{
	{Name: "mint", IsActive: true},
	{Name: "transfer", IsActive: true},
	{Name: "execute", IsActive: true},
	{Name: "deploy", IsActive: true},
	{Name: "burn", IsActive: true},
	{Name: "commit", IsActive: true},
	{Name: "uncommit", IsActive: true},
	{Name: "pledge", IsActive: true},
	{Name: "unpledge", IsActive: true},
}

// GetTokenRoleID returns the numeric ID for a string role name (1-indexed)
func GetTokenRoleID(tokenRole string) int16 {
	for idx, entry := range TokenRoleTypes {
		if entry.Name == tokenRole {
			return int16(idx + 1)
		}
	}
	return -1
}
