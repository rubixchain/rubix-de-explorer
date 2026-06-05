package util

import (
	"testing"
)

func TestIsValidDID(t *testing.T) {
	tests := []struct {
		name     string
		did      string
		expected bool
	}{
		{
			name:     "Valid DID",
			did:      "bafybmihy4panvvrjssdjqksrwjcxza6xpgnxvcyufn2wuam75idnqlugdq",
			expected: true,
		},
		{
			name:     "Invalid DID - too short",
			did:      "bafybmf6j7n5e4v4z7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v",
			expected: false,
		},
		{
			name:     "Invalid DID - wrong prefix",
			did:      "cafybmihy4panvvrjssdjqksrwjcxza6xpgnxvcyufn2wuam75idnqlugdq",
			expected: false,
		},
		{
			name:     "Invalid DID - invalid characters",
			did:      "bafybmihy4panvvrjssdjqksrwjcxza6xpgnxvcyufn2wuam75idnqlugd1", // '1' is not in base32
			expected: false,
		},
		{
			name:     "Empty DID",
			did:      "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsValidDID(tt.did); got != tt.expected {
				t.Errorf("IsValidDID() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestIsValidFT(t *testing.T) {
	const did = "bafybmihy4panvvrjssdjqksrwjcxza6xpgnxvcyufn2wuam75idnqlugdq"
	tests := []struct {
		name     string
		tokenID  string
		expected bool
	}{
		{"Valid FT - simple alphanumeric name", "APPLE_" + did + "_1", true},
		{"Valid FT - name with hyphens (real wire shape)", "ft-A-00001-x7j6_" + did + "_0", true},
		{"Valid FT - name with dots", "Foo.Bar_" + did + "_5", true},
		{"Valid FT - large index", "X_" + did + "_999999", true},
		{"Invalid FT - empty name", "_" + did + "_1", false},
		{"Invalid FT - name contains underscore", "AP_PLE_" + did + "_1", false},
		{"Invalid FT - DID malformed", "APPLE_not-a-did_1", false},
		{"Invalid FT - missing index", "APPLE_" + did, false},
		{"Invalid FT - non-numeric index", "APPLE_" + did + "_abc", false},
		{"Invalid FT - DID uppercase chars (outside base32)", "APPLE_BAFYBMIHY4PANVVRJSSDJQKSRWJCXZA6XPGNXVCYUFN2WUAM75IDNQLUGDQ_1", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsValidFT(tt.tokenID); got != tt.expected {
				t.Errorf("IsValidFT(%q) = %v, want %v", tt.tokenID, got, tt.expected)
			}
		})
	}
}
