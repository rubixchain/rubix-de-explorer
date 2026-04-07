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
	tests := []struct {
		name     string
		tokenID  string
		expected bool
	}{
		{
			name:     "Valid FT",
			tokenID:  "APPLE_1_bafybmihy4panvvrjssdjqksrwjcxza6xpgnxvcyufn2wuam75idnqlugdq",
			expected: true,
		},
		{
			name:     "Invalid FT - no name",
			tokenID:  "_1_bafybmihy4panvvrjssdjqksrwjcxza6xpgnxvcyufn2wuam75idnqlugdq",
			expected: false,
		},
		{
			name:     "Invalid FT - invalid DID",
			tokenID:  "APPLE_1_invalid_did",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsValidFT(tt.tokenID); got != tt.expected {
				t.Errorf("IsValidFT() = %v, want %v", got, tt.expected)
			}
		})
	}
}
