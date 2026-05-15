package util

import (
	"regexp"
)

var (
	// DID: Exactly 59 characters, starts with bafy, CIDv1 Base32
	didRegex = regexp.MustCompile(`^bafy[abcdefghijklmnopqrstuvwxyz234567]{55}$`)

	// RBT: Numeric levels and numbers separated by underscores
	rbtRegex = regexp.MustCompile(`^\d+_\d+(_\d+)*$`)

	// FT: {Name}_{CreatorDID}_{Index}
	ftRegex = regexp.MustCompile(`^[a-zA-Z0-9]+_bafy[abcdefghijklmnopqrstuvwxyz234567]{55}_\d+$`)

	// NFT & Private SC: Must start with Qm, exactly 46 characters, Base58
	ipfsRegex = regexp.MustCompile(`^Qm[1-9A-HJ-NP-Za-km-z]{44}$`)
)

// IsValidDID checks if the provided string is a valid Rubix DID.
func IsValidDID(did string) bool {
	return didRegex.MatchString(did)
}

// IsValidRBT checks if the provided string is a valid RBT TokenID.
func IsValidRBT(tokenID string) bool {
	return rbtRegex.MatchString(tokenID)
}

// IsValidFT checks if the provided string is a valid FT TokenID.
func IsValidFT(tokenID string) bool {
	return ftRegex.MatchString(tokenID)
}

// IsValidNFT checks if the provided string is a valid NFT TokenID.
func IsValidNFT(tokenID string) bool {
	return ipfsRegex.MatchString(tokenID)
}

// IsValidSC checks if the provided string is a valid Smart Contract TokenID.
func IsValidSC(tokenID string) bool {
	return ipfsRegex.MatchString(tokenID)
}

// IsValidTransactionID checks whether transaction IDs are in accepted Rubix formats.
func IsValidTransactionID(txnID string) bool {
	return len(txnID) == 64
}
