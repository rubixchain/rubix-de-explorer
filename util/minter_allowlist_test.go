package util

import "testing"

func TestIsAuthorizedTokenRange_MainnetBoundaries(t *testing.T) {
	tests := []struct {
		name      string
		level     int
		number    int
		wantValid bool
	}{
		{"min number, level 1", 1, 1, true},
		{"max number, level 1", 1, 4_300_000, true},
		{"mid number, level 1", 1, 2_000_000, true},
		{"zero number rejected", 1, 0, false},
		{"negative number rejected", 1, -1, false},
		{"number above max rejected", 1, 4_300_001, false},
		{"wrong level (0)", 0, 100, false},
		{"wrong level (2)", 2, 100, false},
		{"way wrong level (10000)", 10000, 100, false},
		{"testnet level on mainnet rejected", 50001, 100, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsAuthorizedTokenRange(NetworkMainnet, tt.level, tt.number)
			if got != tt.wantValid {
				t.Errorf("got %v, want %v", got, tt.wantValid)
			}
		})
	}
}

func TestIsAuthorizedTokenRange_TestnetBoundaries(t *testing.T) {
	tests := []struct {
		name      string
		level     int
		number    int
		wantValid bool
	}{
		{"level 50001 min", 50001, 1, true},
		{"level 50001 max", 50001, 4_300_000, true},
		{"level 50000 (one below) rejected", 50000, 100, false},
		{"level 50002 (one above) rejected", 50002, 100, false},
		{"level 50003 rejected", 50003, 100, false},
		{"level 50004 rejected", 50004, 100, false},
		{"mainnet level on testnet rejected", 1, 100, false},
		{"number above max rejected", 50001, 4_300_001, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsAuthorizedTokenRange(NetworkTestnet, tt.level, tt.number)
			if got != tt.wantValid {
				t.Errorf("got %v, want %v", got, tt.wantValid)
			}
		})
	}
}

func TestIsAuthorizedTokenRange_UnknownNetworkRejected(t *testing.T) {
	if IsAuthorizedTokenRange("localnet", 1, 100) {
		t.Error("localnet should not be authorized")
	}
	if IsAuthorizedTokenRange("", 1, 100) {
		t.Error("empty network should not be authorized")
	}
}

func TestIsAuthorizedTokenRange_NetworkCaseInsensitive(t *testing.T) {
	for _, n := range []string{"MAINNET", "MainNet", "  mainnet  "} {
		if !IsAuthorizedTokenRange(n, 1, 100) {
			t.Errorf("%q should be normalized to mainnet", n)
		}
	}
}

func TestIsAuthorizedMint_MainnetAllElevenDIDsAtBoundaries(t *testing.T) {
	for _, e := range mainnetAllowlist {
		t.Run(e.DID[:16], func(t *testing.T) {
			if !IsAuthorizedMint(NetworkMainnet, e.DID, e.Level, e.StartTokenNumber) {
				t.Errorf("DID %s: start number %d should be authorized", e.DID, e.StartTokenNumber)
			}
			if !IsAuthorizedMint(NetworkMainnet, e.DID, e.Level, e.EndTokenNumber) {
				t.Errorf("DID %s: end number %d should be authorized", e.DID, e.EndTokenNumber)
			}
			if IsAuthorizedMint(NetworkMainnet, e.DID, e.Level, e.StartTokenNumber-1) {
				t.Errorf("DID %s: %d is one below start, should be rejected", e.DID, e.StartTokenNumber-1)
			}
			if IsAuthorizedMint(NetworkMainnet, e.DID, e.Level, e.EndTokenNumber+1) {
				t.Errorf("DID %s: %d is one above end, should be rejected", e.DID, e.EndTokenNumber+1)
			}
		})
	}
}

func TestIsAuthorizedMint_MainnetWrongDID(t *testing.T) {
	stranger := "bafybmibogusbogusbogusbogusbogusbogusbogusbogusbogusbogusbg"
	if IsAuthorizedMint(NetworkMainnet, stranger, 1, 100) {
		t.Error("unknown DID on mainnet should be rejected")
	}
}

func TestIsAuthorizedMint_MainnetWrongLevel(t *testing.T) {
	entry := mainnetAllowlist[0]
	if IsAuthorizedMint(NetworkMainnet, entry.DID, 2, entry.StartTokenNumber) {
		t.Error("mainnet allowlisted DID at wrong level should be rejected")
	}
}

func TestIsAuthorizedMint_MainnetOutOfRange(t *testing.T) {
	entry := mainnetAllowlist[0]
	if IsAuthorizedMint(NetworkMainnet, entry.DID, entry.Level, entry.EndTokenNumber+10_000) {
		t.Error("mainnet allowlisted DID outside its range should be rejected")
	}
}

func TestIsAuthorizedMint_MainnetDifferentDIDForRange(t *testing.T) {
	row1DID := mainnetAllowlist[0].DID
	row2Range := mainnetAllowlist[1]
	if IsAuthorizedMint(NetworkMainnet, row1DID, row2Range.Level, row2Range.StartTokenNumber) {
		t.Error("row 1 DID minting in row 2's range should be rejected")
	}
}

func TestIsAuthorizedMint_TestnetAllFourDIDsAtBoundaries(t *testing.T) {
	for _, e := range testnetAllowlist {
		t.Run(e.DID[:16], func(t *testing.T) {
			if !IsAuthorizedMint(NetworkTestnet, e.DID, e.Level, e.StartTokenNumber) {
				t.Errorf("DID %s: start %d should be authorized", e.DID, e.StartTokenNumber)
			}
			if !IsAuthorizedMint(NetworkTestnet, e.DID, e.Level, e.EndTokenNumber) {
				t.Errorf("DID %s: end %d should be authorized", e.DID, e.EndTokenNumber)
			}
		})
	}
}

// Row 1 covers 1..2,000,000; row 2 covers 1..1,000,000 — both DIDs share
// the 1..1,000,000 window, but only row 1 covers 1,000,001..2,000,000.
func TestIsAuthorizedMint_TestnetRow1Row2Overlap_BothDIDsAuthorized(t *testing.T) {
	row1 := testnetAllowlist[0]
	row2 := testnetAllowlist[1]
	const insideOverlap = 500_000
	if !IsAuthorizedMint(NetworkTestnet, row1.DID, 50001, insideOverlap) {
		t.Errorf("row 1 DID should mint in the overlap (%d)", insideOverlap)
	}
	if !IsAuthorizedMint(NetworkTestnet, row2.DID, 50001, insideOverlap) {
		t.Errorf("row 2 DID should mint in the overlap (%d)", insideOverlap)
	}
	const aboveRow2 = 1_500_000
	if !IsAuthorizedMint(NetworkTestnet, row1.DID, 50001, aboveRow2) {
		t.Errorf("row 1 DID should mint at %d (within row 1's range)", aboveRow2)
	}
	if IsAuthorizedMint(NetworkTestnet, row2.DID, 50001, aboveRow2) {
		t.Errorf("row 2 DID must NOT mint at %d (above row 2's end)", aboveRow2)
	}
}

func TestIsAuthorizedMint_TestnetUnknownDID_Rejected(t *testing.T) {
	stranger := "bafybmistrangermistrangermistrangermistrangermistrangerff"
	if IsAuthorizedMint(NetworkTestnet, stranger, 50001, 100) {
		t.Error("unknown DID on testnet should be rejected")
	}
}

// Use an allowlisted DID so the rejection is unambiguously about level, not DID.
func TestIsAuthorizedMint_TestnetWrongLevel(t *testing.T) {
	allowed := testnetAllowlist[0].DID
	if IsAuthorizedMint(NetworkTestnet, allowed, 1, 100) {
		t.Error("testnet allowlisted DID at mainnet level should be rejected")
	}
	if IsAuthorizedMint(NetworkTestnet, allowed, 50002, 100) {
		t.Error("testnet allowlisted DID at level 50002 should be rejected")
	}
	if IsAuthorizedMint(NetworkTestnet, allowed, 10000, 100) {
		t.Error("testnet allowlisted DID at level 10000 should be rejected")
	}
}

func TestIsAuthorizedMint_TestnetDIDMintingOtherPartition_Rejected(t *testing.T) {
	row1DID := testnetAllowlist[0].DID
	row3 := testnetAllowlist[2]
	if IsAuthorizedMint(NetworkTestnet, row1DID, 50001, row3.StartTokenNumber) {
		t.Error("row 1 DID must not mint in row 3's partition")
	}
}

func TestIsAuthorizedMint_TestnetNumberOutOfRange(t *testing.T) {
	allowed := testnetAllowlist[3].DID
	if IsAuthorizedMint(NetworkTestnet, allowed, 50001, 99_999_999) {
		t.Error("testnet at out-of-range number should be rejected")
	}
}

func TestIsAuthorizedMint_UnknownNetwork(t *testing.T) {
	if IsAuthorizedMint("localnet", "anyDID", 1, 100) {
		t.Error("localnet should always be rejected")
	}
}

func TestNormalizeNetwork(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"mainnet", "mainnet"},
		{"MainNet", "mainnet"},
		{"  TESTNET ", "testnet"},
		{"localnet", ""},
		{"", ""},
	}
	for _, tt := range tests {
		if got := NormalizeNetwork(tt.in); got != tt.want {
			t.Errorf("NormalizeNetwork(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
