package processor

import (
	"explorer-server/model"
	"testing"
)

func TestValidateTransactionFormat_DIDs(t *testing.T) {
	validDID := "bafybmihy4panvvrjssdjqksrwjcxza6xpgnxvcyufn2wuam75idnqlugdq"
	invalidDID := "invalid_did_format"

	tests := []struct {
		name     string
		txn      *model.EventTransaction
		expected bool
	}{
		{
			name: "Valid Transaction All DIDs",
			txn: &model.EventTransaction{
				Transaction: &model.Transactions{
					TransactionID: "TX123",
					TransactionInfo: &model.TransactionInfo{
						Initiator: validDID,
						Owner:     validDID,
						Quorums: []*model.QuorumInfo{
							{Did: validDID},
						},
					},
					Signatures: &model.Signature{
						Quorums: []model.QuorumSignature{
							{Did: validDID, Signature: "sig1"},
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "Invalid Quorum DID",
			txn: &model.EventTransaction{
				Transaction: &model.Transactions{
					TransactionID: "TX124",
					TransactionInfo: &model.TransactionInfo{
						Initiator: validDID,
						Owner:     validDID,
						Quorums: []*model.QuorumInfo{
							{Did: invalidDID},
						},
					},
				},
			},
			expected: false,
		},
		{
			name: "Invalid Quorum Signature DID",
			txn: &model.EventTransaction{
				Transaction: &model.Transactions{
					TransactionID: "TX125",
					TransactionInfo: &model.TransactionInfo{
						Initiator: validDID,
						Owner:     validDID,
						Quorums: []*model.QuorumInfo{
							{Did: validDID},
						},
					},
					Signatures: &model.Signature{
						Quorums: []model.QuorumSignature{
							{Did: invalidDID, Signature: "sig1"},
						},
					},
				},
			},
			expected: false,
		},
		{
			name: "Invalid Initiator DID",
			txn: &model.EventTransaction{
				Transaction: &model.Transactions{
					TransactionID: "TX126",
					TransactionInfo: &model.TransactionInfo{
						Initiator: invalidDID,
						Owner:     validDID,
					},
				},
			},
			expected: false,
		},
		{
			name: "Invalid Owner DID",
			txn: &model.EventTransaction{
				Transaction: &model.Transactions{
					TransactionID: "TX127",
					TransactionInfo: &model.TransactionInfo{
						Initiator: validDID,
						Owner:     invalidDID,
					},
				},
			},
			expected: false,
		},
		{
			name: "Invalid RBT TokenID",
			txn: &model.EventTransaction{
				Transaction: &model.Transactions{
					TransactionID: "TX128",
					TransactionInfo: &model.TransactionInfo{
						Tokens: &model.TransactionTokens{
							RBT: []*model.TokenInfo{
								{TokenID: "invalid_rbt"},
							},
						},
					},
				},
			},
			expected: false,
		},
		{
			name: "Invalid FT TokenID (Missing Name)",
			txn: &model.EventTransaction{
				Transaction: &model.Transactions{
					TransactionID: "TX129",
					TransactionInfo: &model.TransactionInfo{
						Tokens: &model.TransactionTokens{
							FT: []*model.TokenInfo{
								{TokenID: "_1_" + validDID},
							},
						},
					},
				},
			},
			expected: false,
		},
		{
			name: "Invalid FT TokenID (Invalid HID)",
			txn: &model.EventTransaction{
				Transaction: &model.Transactions{
					TransactionID: "TX130",
					TransactionInfo: &model.TransactionInfo{
						Tokens: &model.TransactionTokens{
							FT: []*model.TokenInfo{
								{TokenID: "APPLE_1_" + invalidDID},
							},
						},
					},
				},
			},
			expected: false,
		},
		{
			name: "Invalid RBT in CommittedTokens",
			txn: &model.EventTransaction{
				Transaction: &model.Transactions{
					TransactionID: "TX131",
					TransactionInfo: &model.TransactionInfo{
						CommittedTokens: []*model.TokenInfo{
							{TokenID: "invalid_rbt"},
						},
					},
				},
			},
			expected: false,
		},
		{
			name: "Invalid RBT in Quorum Tokens",
			txn: &model.EventTransaction{
				Transaction: &model.Transactions{
					TransactionID: "TX132",
					TransactionInfo: &model.TransactionInfo{
						Quorums: []*model.QuorumInfo{
							{
								Did: validDID,
								Tokens: []*model.TokenInfo{
									{TokenID: "invalid_rbt"},
								},
							},
						},
					},
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ValidateTransactionFormat(tt.txn); got != tt.expected {
				t.Errorf("ValidateTransactionFormat() = %v, want %v", got, tt.expected)
			}
		})
	}
}
