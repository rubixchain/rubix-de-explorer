package util

import "strings"

// IsValidDID checks if the provided string is a valid Rubix DID.
func IsValidDID(did string) bool {
	// Rubix core validation pattern: prefix "bafybmi", length 59, base32-like charset.
	if len(did) != 59 || !strings.HasPrefix(did, "bafybmi") {
		return false
	}
	for i := 0; i < len(did); i++ {
		c := did[i]
		if (c >= 'a' && c <= 'z') || (c >= '2' && c <= '7') {
			continue
		}
		return false
	}
	return true
}

// IsValidRBT checks if the provided string is a valid RBT TokenID.
func IsValidRBT(tokenID string) bool {
	if tokenID == "" {
		return false
	}
	hasDigit := false
	prevUnderscore := false
	for i := 0; i < len(tokenID); i++ {
		c := tokenID[i]
		if c >= '0' && c <= '9' {
			hasDigit = true
			prevUnderscore = false
			continue
		}
		if c == '_' {
			if i == 0 || i == len(tokenID)-1 || prevUnderscore {
				return false
			}
			prevUnderscore = true
			continue
		}
		return false
	}
	return hasDigit
}

// IsValidFT checks if the provided string is a valid FT TokenID.
func IsValidFT(tokenID string) bool {
	// Format: {Name}_{Index}_{CreatorDID}
	first := strings.IndexByte(tokenID, '_')
	last := strings.LastIndexByte(tokenID, '_')
	if first <= 0 || last <= first+1 || last >= len(tokenID)-1 {
		return false
	}
	name := tokenID[:first]
	index := tokenID[first+1 : last]
	creatorDID := tokenID[last+1:]

	// Name should be alphanumeric.
	for i := 0; i < len(name); i++ {
		c := name[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') {
			continue
		}
		return false
	}
	// Index should be digits only.
	for i := 0; i < len(index); i++ {
		c := index[i]
		if c < '0' || c > '9' {
			return false
		}
	}
	return IsValidDID(creatorDID)
}

// IsValidNFT checks if the provided string is a valid NFT TokenID.
func IsValidNFT(tokenID string) bool {
	// Rubix NFT IDs are IPFS CIDv0 style: Qm + 44 base58 chars.
	if len(tokenID) != 46 || tokenID[0] != 'Q' || tokenID[1] != 'm' {
		return false
	}
	for i := 2; i < len(tokenID); i++ {
		if !isBase58Char(tokenID[i]) {
			return false
		}
	}
	return true
}

// IsValidSC checks if the provided string is a valid Smart Contract TokenID.
func IsValidSC(tokenID string) bool {
	// SC IDs follow same CIDv0 format as NFT IDs.
	return IsValidNFT(tokenID)
}

// IsValidTransactionID checks whether transaction IDs are in accepted Rubix formats.
func IsValidTransactionID(txnID string) bool {
	// Rubix core generates transaction IDs as SHA3-256 hex strings.
	if len(txnID) != 64 {
		return false
	}
	for i := 0; i < len(txnID); i++ {
		c := txnID[i]
		if (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') {
			continue
		}
		return false
	}
	return true
}

func isBase58Char(c byte) bool {
	switch {
	case c >= '1' && c <= '9':
		return true
	case c >= 'A' && c <= 'H':
		return true
	case c >= 'J' && c <= 'N':
		return true
	case c >= 'P' && c <= 'Z':
		return true
	case c >= 'a' && c <= 'k':
		return true
	case c >= 'm' && c <= 'z':
		return true
	default:
		return false
	}
}
