package processor

import (
	"encoding/json"
	"explorer-server/model"
	"testing"
)

// buildTestTxn constructs a model.Transactions with raw JSON Info and Signature,
// matching how the Rubix Core actually publishes data.
func buildTestTxn(id string, info *model.TransactionInfo, sig *model.Signature) *model.Transactions {
	var infoBytes, sigBytes json.RawMessage
	if info != nil {
		infoBytes, _ = json.Marshal(info)
	}
	if sig != nil {
		sigBytes, _ = json.Marshal(sig)
	}
	return &model.Transactions{
		ID:        id,
		Info:      infoBytes,
		Signature: sigBytes,
	}
}

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
				Transaction: buildTestTxn("TX123",
					&model.TransactionInfo{
						Initiator: validDID,
						Owner:     validDID,
						Quorums: []*model.QuorumInfo{
							{Did: validDID},
						},
					},
					&model.Signature{
						Quorums: []model.QuorumSignature{
							{Did: validDID, Signature: "sig1"},
						},
					},
				),
			},
			expected: true,
		},
		{
			name: "Invalid Quorum DID",
			txn: &model.EventTransaction{
				Transaction: buildTestTxn("TX124",
					&model.TransactionInfo{
						Initiator: validDID,
						Owner:     validDID,
						Quorums: []*model.QuorumInfo{
							{Did: invalidDID},
						},
					},
					nil,
				),
			},
			expected: false,
		},
		{
			name: "Invalid Quorum Signature DID",
			txn: &model.EventTransaction{
				Transaction: buildTestTxn("TX125",
					&model.TransactionInfo{
						Initiator: validDID,
						Owner:     validDID,
						Quorums: []*model.QuorumInfo{
							{Did: validDID},
						},
					},
					&model.Signature{
						Quorums: []model.QuorumSignature{
							{Did: invalidDID, Signature: "sig1"},
						},
					},
				),
			},
			expected: false,
		},
		{
			name: "Invalid Initiator DID",
			txn: &model.EventTransaction{
				Transaction: buildTestTxn("TX126",
					&model.TransactionInfo{
						Initiator: invalidDID,
						Owner:     validDID,
					},
					nil,
				),
			},
			expected: false,
		},
		{
			name: "Invalid Owner DID",
			txn: &model.EventTransaction{
				Transaction: buildTestTxn("TX127",
					&model.TransactionInfo{
						Initiator: validDID,
						Owner:     invalidDID,
					},
					nil,
				),
			},
			expected: false,
		},
		{
			name: "Invalid RBT TokenID",
			txn: &model.EventTransaction{
				Transaction: buildTestTxn("TX128",
					&model.TransactionInfo{
						Tokens: &model.TransactionTokens{
							RBT: []*model.TokenInfo{
								{TokenID: "invalid_rbt"},
							},
						},
					},
					nil,
				),
			},
			expected: false,
		},
		{
			name: "Invalid FT TokenID (Missing Name)",
			txn: &model.EventTransaction{
				Transaction: buildTestTxn("TX129",
					&model.TransactionInfo{
						Tokens: &model.TransactionTokens{
							FT: []*model.TokenInfo{
								{TokenID: "_1_" + validDID},
							},
						},
					},
					nil,
				),
			},
			expected: false,
		},
		{
			name: "Invalid FT TokenID (Invalid HID)",
			txn: &model.EventTransaction{
				Transaction: buildTestTxn("TX130",
					&model.TransactionInfo{
						Tokens: &model.TransactionTokens{
							FT: []*model.TokenInfo{
								{TokenID: "APPLE_1_" + invalidDID},
							},
						},
					},
					nil,
				),
			},
			expected: false,
		},
		{
			name: "Invalid RBT in CommittedTokens",
			txn: &model.EventTransaction{
				Transaction: buildTestTxn("TX131",
					&model.TransactionInfo{
						CommittedTokens: []*model.TokenInfo{
							{TokenID: "invalid_rbt"},
						},
					},
					nil,
				),
			},
			expected: false,
		},
		{
			name: "Invalid RBT in Quorum Tokens",
			txn: &model.EventTransaction{
				Transaction: buildTestTxn("TX132",
					&model.TransactionInfo{
						Quorums: []*model.QuorumInfo{
							{
								Did: validDID,
								Tokens: []*model.TokenInfo{
									{TokenID: "invalid_rbt"},
								},
							},
						},
					},
					nil,
				),
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
