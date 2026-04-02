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

// Token Status
const (
	TokenStatus_Burned int16 = 0
	TokenStatus_Active int16 = 1
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
