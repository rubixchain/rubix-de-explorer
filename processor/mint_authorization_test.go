package processor

import (
	"strings"
	"testing"

	"explorer-server/model"
)

const (
	mainnetMinterRow1DID = "bafybmid35wgktknwpaddfxkgdrouq5z2cmcdz27gf2u4xvrh4pswmrnvpe"
	mainnetMinterRow2DID = "bafybmifogacbfyny7wahapjuk7jpjl3ffylsakmtbrcqmdwckofnsg6rbu"
	testnetMinterRow1DID = "bafybmiairxfiplfpwvzzgslubbx3dwqrutwhrgtypnzyb752rrjbthgrwm"
)

// rbtSpec format: "tokenID|previous_transaction_id"; empty prev = mint.
func mkInfo(network, initiator string, rbtSpecs ...string) *model.TransactionInfo {
	rbts := make([]*model.TokenInfo, 0, len(rbtSpecs))
	for _, spec := range rbtSpecs {
		parts := strings.SplitN(spec, "|", 2)
		t := &model.TokenInfo{TokenID: parts[0]}
		if len(parts) == 2 {
			t.PreviousTransactionID = parts[1]
		}
		rbts = append(rbts, t)
	}
	return &model.TransactionInfo{
		Network:   network,
		Initiator: initiator,
		Tokens:    &model.TransactionTokens{RBT: rbts},
	}
}

func TestValidateAllowlist_NetworkMismatch_Rejected(t *testing.T) {
	SetExplorerNetwork(false)
	defer resetExplorerNetworkForTesting()

	info := mkInfo("testnet", mainnetMinterRow1DID, "1_100|")
	ok, reason := ValidateAllowlist(info)
	if ok {
		t.Fatal("testnet transaction on mainnet explorer should be rejected")
	}
	if !strings.Contains(reason, "network mismatch") {
		t.Errorf("reason should call out network mismatch, got %q", reason)
	}
}

// Motivating scenario: localnet user with testnet swarm key mints
// level-10000 tokens; transactions bleed into the testnet explorer via
// shared IPFS pubsub. info.Network is "testnet" so layer 1 passes; layer 2
// catches the level.
func TestValidateAllowlist_NetworkLeakFromLocalnet_Rejected(t *testing.T) {
	SetExplorerNetwork(true)
	defer resetExplorerNetworkForTesting()

	info := mkInfo("testnet", "anyDID", "10000_500|")
	ok, reason := ValidateAllowlist(info)
	if ok {
		t.Fatal("level-10000 token on testnet explorer should be rejected")
	}
	if !strings.Contains(reason, "outside allowed range") {
		t.Errorf("reason should call out range mismatch, got %q", reason)
	}
}

func TestValidateAllowlist_NetworkUnconfigured_Rejected(t *testing.T) {
	resetExplorerNetworkForTesting()
	info := mkInfo("mainnet", mainnetMinterRow1DID, "1_100|")
	ok, _ := ValidateAllowlist(info)
	if ok {
		t.Error("unconfigured explorer should reject everything")
	}
}

func TestValidateAllowlist_EmptyInfoNetwork_Rejected(t *testing.T) {
	SetExplorerNetwork(false)
	defer resetExplorerNetworkForTesting()

	info := mkInfo("", mainnetMinterRow1DID, "1_100|")
	ok, reason := ValidateAllowlist(info)
	if ok {
		t.Error("empty info.Network should be rejected")
	}
	if !strings.Contains(reason, "network mismatch") {
		t.Errorf("reason should mention network mismatch, got %q", reason)
	}
}

func TestValidateAllowlist_MainnetTokenOutOfNumberRange_Rejected(t *testing.T) {
	SetExplorerNetwork(false)
	defer resetExplorerNetworkForTesting()

	info := mkInfo("mainnet", mainnetMinterRow1DID, "1_99999999|")
	ok, reason := ValidateAllowlist(info)
	if ok {
		t.Fatal("out-of-range token number should be rejected on mainnet")
	}
	if !strings.Contains(reason, "outside allowed range") {
		t.Errorf("reason should mention range, got %q", reason)
	}
}

func TestValidateAllowlist_MainnetWrongLevel_Rejected(t *testing.T) {
	SetExplorerNetwork(false)
	defer resetExplorerNetworkForTesting()

	info := mkInfo("mainnet", mainnetMinterRow1DID, "2_100|")
	ok, _ := ValidateAllowlist(info)
	if ok {
		t.Error("level-2 token on mainnet should be rejected")
	}
}

func TestValidateAllowlist_TestnetAuthorizedMint_Accepted(t *testing.T) {
	SetExplorerNetwork(true)
	defer resetExplorerNetworkForTesting()

	info := mkInfo("testnet", testnetMinterRow1DID, "50001_100|")
	ok, reason := ValidateAllowlist(info)
	if !ok {
		t.Errorf("authorized testnet mint should be accepted, got %q", reason)
	}
}

func TestValidateAllowlist_TestnetUnauthorizedDIDMint_Rejected(t *testing.T) {
	SetExplorerNetwork(true)
	defer resetExplorerNetworkForTesting()

	stranger := "bafybmistrangermistrangermistrangermistrangermistrangerff"
	info := mkInfo("testnet", stranger, "50001_100|")
	ok, reason := ValidateAllowlist(info)
	if ok {
		t.Fatal("unknown DID mint should be rejected on testnet")
	}
	if !strings.Contains(reason, "unauthorized mint") {
		t.Errorf("reason should mention unauthorized mint, got %q", reason)
	}
}

func TestValidateAllowlist_TestnetTransferByAnyDID_Accepted(t *testing.T) {
	SetExplorerNetwork(true)
	defer resetExplorerNetworkForTesting()

	info := mkInfo("testnet", "bafybmianyownerownerownerownerownerownerownerownerownerYAA",
		"50001_100|prev_tx_id")
	ok, reason := ValidateAllowlist(info)
	if !ok {
		t.Errorf("valid testnet transfer by any DID should be accepted, got %q", reason)
	}
}

func TestValidateAllowlist_TestnetNonAllowedLevel_Rejected(t *testing.T) {
	SetExplorerNetwork(true)
	defer resetExplorerNetworkForTesting()

	for _, lvl := range []string{"50002", "50005", "10000"} {
		info := mkInfo("testnet", testnetMinterRow1DID, lvl+"_100|")
		ok, _ := ValidateAllowlist(info)
		if ok {
			t.Errorf("level %s should be rejected on testnet", lvl)
		}
	}
}

func TestValidateAllowlist_MainnetAuthorizedMint_Accepted(t *testing.T) {
	SetExplorerNetwork(false)
	defer resetExplorerNetworkForTesting()

	info := mkInfo("mainnet", mainnetMinterRow1DID, "1_100|")
	ok, reason := ValidateAllowlist(info)
	if !ok {
		t.Errorf("authorized mainnet mint should be accepted, got %q", reason)
	}
}

func TestValidateAllowlist_MainnetUnauthorizedDIDMint_Rejected(t *testing.T) {
	SetExplorerNetwork(false)
	defer resetExplorerNetworkForTesting()

	stranger := "bafybmistrangermistrangermistrangermistrangermistrangerff"
	info := mkInfo("mainnet", stranger, "1_100|")
	ok, reason := ValidateAllowlist(info)
	if ok {
		t.Fatal("unknown DID mint should be rejected on mainnet")
	}
	if !strings.Contains(reason, "unauthorized mint") {
		t.Errorf("reason should mention unauthorized mint, got %q", reason)
	}
}

func TestValidateAllowlist_MainnetDIDMintingOtherPartition_Rejected(t *testing.T) {
	SetExplorerNetwork(false)
	defer resetExplorerNetworkForTesting()

	info := mkInfo("mainnet", mainnetMinterRow1DID, "1_500000|")
	ok, _ := ValidateAllowlist(info)
	if ok {
		t.Error("row-1 DID minting in row-2 range should be rejected")
	}
}

func TestValidateAllowlist_MainnetAuthorizedTransfer_Accepted(t *testing.T) {
	SetExplorerNetwork(false)
	defer resetExplorerNetworkForTesting()

	info := mkInfo("mainnet", "bafybmianyowner-thesemattersnotforthecheckanyownerYAA", "1_100|prev_tx_id")
	ok, reason := ValidateAllowlist(info)
	if !ok {
		t.Errorf("valid transfer by any DID should be accepted, got %q", reason)
	}
}

func TestValidateAllowlist_MainnetTransferWithBadTokenRange_Rejected(t *testing.T) {
	SetExplorerNetwork(false)
	defer resetExplorerNetworkForTesting()

	info := mkInfo("mainnet", "anyDID", "10000_500|prev_tx_id")
	ok, _ := ValidateAllowlist(info)
	if ok {
		t.Error("transfer of out-of-range token should be rejected")
	}
}

// One non-empty prev in the RBT slice means the transaction is classified
// as a transfer — DID gate is bypassed for all its tokens. Matches the
// pubsub-path classifier in token_operations.go.
func TestValidateAllowlist_MixedRBT_OneTransferDisablesMintCheck(t *testing.T) {
	SetExplorerNetwork(false)
	defer resetExplorerNetworkForTesting()

	info := mkInfo("mainnet",
		"bafybmistrangermistrangermistrangermistrangermistrangerff",
		"1_100|",
		"1_200|prev_tx_id",
	)
	ok, _ := ValidateAllowlist(info)
	if !ok {
		t.Error("mixed mint+transfer should pass DID gate (isn't classified as mint)")
	}
}

func TestValidateAllowlist_NoRBT_OnlyChecksNetwork(t *testing.T) {
	SetExplorerNetwork(false)
	defer resetExplorerNetworkForTesting()

	info := &model.TransactionInfo{
		Network:   "mainnet",
		Initiator: "anyDID",
		Tokens:    &model.TransactionTokens{},
	}
	ok, reason := ValidateAllowlist(info)
	if !ok {
		t.Errorf("no-RBT transaction with valid network should pass, got %q", reason)
	}
}

func TestValidateAllowlist_NilInfo_Passes(t *testing.T) {
	ok, _ := ValidateAllowlist(nil)
	if !ok {
		t.Error("nil info should not be rejected by allowlist")
	}
}

func TestValidateAllowlist_ReasonsContainKeyFields(t *testing.T) {
	SetExplorerNetwork(false)
	defer resetExplorerNetworkForTesting()

	cases := []struct {
		name       string
		info       *model.TransactionInfo
		wantTokens []string
	}{
		{
			"out-of-range token surfaced",
			mkInfo("mainnet", mainnetMinterRow1DID, "1_99999999|"),
			[]string{"1_99999999", "level=1", "number=99999999"},
		},
		{
			"unauthorized mint surfaces DID + token",
			mkInfo("mainnet", "bafybmistrangermistrangermistrangermistrangermistrangerff", "1_100|"),
			[]string{"unauthorized mint", "bafybmistranger", "1_100"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ok, reason := ValidateAllowlist(tc.info)
			if ok {
				t.Fatal("expected rejection")
			}
			for _, want := range tc.wantTokens {
				if !strings.Contains(reason, want) {
					t.Errorf("reason missing %q (got: %q)", want, reason)
				}
			}
		})
	}
}
