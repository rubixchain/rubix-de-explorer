package processor

import (
	"encoding/json"
	"strings"
	"testing"

	"explorer-server/model"
)

const (
	validInitiatorDID = "bafybmic277e6pdjd5cuyetuwl467holr5bosy3l7oz6ey6h7yrotbhka44"
	validOwnerDID     = "bafybmidclepelwoxrgpttfeeffzfydk464ppd5pnmelxvihxr2gud5cwrq"
	validTxnID        = "428114eaf0209abeeed0fae44d1e71ff1b618f782e0e554b36e008a4ba3084ad"
)

func mkEvent(rawInfo string) *model.EventTransaction {
	return &model.EventTransaction{
		Transaction: &model.Transactions{
			ID:   validTxnID,
			Info: json.RawMessage(rawInfo),
		},
	}
}

// Regression: a whitespace-padded owner DID (seen from upstream) must not be
// rejected as "invalid owner DID format". ParseInfo now trims it, so the
// anchored DID regex passes.
func TestValidateTransactionFormat_WhitespacePaddedOwnerDID_Accepted(t *testing.T) {
	rawInfo := `{"initiator":"` + validInitiatorDID +
		`","owner":"   ` + validOwnerDID + `  ","network":"mainnet"}`

	ok, reason := ValidateTransactionFormatWithReason(mkEvent(rawInfo))
	if !ok {
		t.Fatalf("whitespace-padded owner DID should be accepted after normalization, got %q", reason)
	}
}

// A genuinely malformed DID (wrong length) must still be rejected — trimming
// must not mask real format errors.
func TestValidateTransactionFormat_TrulyInvalidOwnerDID_Rejected(t *testing.T) {
	rawInfo := `{"initiator":"` + validInitiatorDID +
		`","owner":"bafyTOOSHORT","network":"mainnet"}`

	ok, reason := ValidateTransactionFormatWithReason(mkEvent(rawInfo))
	if ok {
		t.Fatal("malformed owner DID should still be rejected")
	}
	if !strings.Contains(reason, "invalid owner DID format") {
		t.Errorf("reason should call out the owner DID, got %q", reason)
	}
}

// ParseInfo normalizes in place: the parsed struct carries trimmed identifiers,
// so the flattened TransactionInfo row and asset processing store clean values.
func TestParseInfo_TrimsDIDsAndTokenIDs(t *testing.T) {
	rawInfo := `{"initiator":"  ` + validInitiatorDID +
		`","owner":"` + validOwnerDID +
		`  ","network":" mainnet ","tokens":{"rbt":[{"tokenId":" 1_795733_433 ","previousTransactionID":" abc "}]}}`

	info, err := mkEvent(rawInfo).Transaction.ParseInfo()
	if err != nil {
		t.Fatalf("ParseInfo: %v", err)
	}
	if info.Initiator != validInitiatorDID {
		t.Errorf("initiator not trimmed: %q", info.Initiator)
	}
	if info.Owner != validOwnerDID {
		t.Errorf("owner not trimmed: %q", info.Owner)
	}
	if info.Network != "mainnet" {
		t.Errorf("network not trimmed: %q", info.Network)
	}
	rbt := info.Tokens.RBT[0]
	if rbt.TokenID != "1_795733_433" {
		t.Errorf("token id not trimmed: %q", rbt.TokenID)
	}
	if rbt.PreviousTransactionID != "abc" {
		t.Errorf("prev txn id not trimmed: %q", rbt.PreviousTransactionID)
	}
}
