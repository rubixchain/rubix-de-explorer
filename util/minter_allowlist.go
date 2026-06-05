package util

import "strings"

// Mirrors rubixgoplatform/core/minterallowlist/mint_access_control.go on
// branch vaishnav/fullnode-sync-API-for-explorer. Keep in sync manually.

type MintAccessRange struct {
	DID              string
	Level            int
	StartTokenNumber int
	EndTokenNumber   int
}

const (
	rbtTokenNumberMin = 1
	rbtTokenNumberMax = 4_300_000
)

const mainnetLevel = 1

var mainnetAllowlist = []MintAccessRange{
	{DID: "bafybmid35wgktknwpaddfxkgdrouq5z2cmcdz27gf2u4xvrh4pswmrnvpe", Level: 1, StartTokenNumber: 1, EndTokenNumber: 400000},
	{DID: "bafybmifogacbfyny7wahapjuk7jpjl3ffylsakmtbrcqmdwckofnsg6rbu", Level: 1, StartTokenNumber: 400001, EndTokenNumber: 800000},
	{DID: "bafybmielnukpgfknvyrsocrvxshplwecjv3bqapt6my3juy4fa6gez4zmy", Level: 1, StartTokenNumber: 800001, EndTokenNumber: 1200000},
	{DID: "bafybmie6vd43nyvzesubkfddpydja6rwlxijsgkcjfsthovlbujclm3td4", Level: 1, StartTokenNumber: 1200001, EndTokenNumber: 1600000},
	{DID: "bafybmieybmtvh22j2kzjr5w4yzptbeax3musw7rcqwz6vkaejemn53qsvi", Level: 1, StartTokenNumber: 1600001, EndTokenNumber: 2000000},
	{DID: "bafybmighj3g2ip7c5wmfzulcfkdo6t4734oekbpeb7mh73nyp4sv4mvyqu", Level: 1, StartTokenNumber: 2000001, EndTokenNumber: 2400000},
	{DID: "bafybmifty4w2ccl3zcclaehgong5vhuocr3pnabccfnn27z2e6knxx7aye", Level: 1, StartTokenNumber: 2400001, EndTokenNumber: 2800000},
	{DID: "bafybmifk4dzbmjl434kdn32u4m4rzr6qgyvu7t4jtql2muqn7lx5fyj6aa", Level: 1, StartTokenNumber: 2800001, EndTokenNumber: 3200000},
	{DID: "bafybmihm6nutigowauas3232pgxhm3sk4xr4o7t7q7j4ejklh4hkmrrjna", Level: 1, StartTokenNumber: 3200001, EndTokenNumber: 3600000},
	{DID: "bafybmibw2b7hokbfvcdqb6u53gbve276oc2x2svaalklgzv4mzgfadw7cm", Level: 1, StartTokenNumber: 3600001, EndTokenNumber: 4000000},
	{DID: "bafybmifmwqhlscye636ui5ajybavpvyfb6gf25m3kkxhm7pewc6slh3yue", Level: 1, StartTokenNumber: 4000001, EndTokenNumber: 4300000},
}

const testnetLevel = 50001

// Row 1 and row 2 overlap on 1..1,000,000 — intentional, matches upstream.
var testnetAllowlist = []MintAccessRange{
	{DID: "bafybmiairxfiplfpwvzzgslubbx3dwqrutwhrgtypnzyb752rrjbthgrwm", Level: 50001, StartTokenNumber: 1, EndTokenNumber: 2000000},
	{DID: "bafybmifduxb6ot7ta6bwdbedhiymuzsfwzgsf6ovb7tdwahacmogczgrha", Level: 50001, StartTokenNumber: 1, EndTokenNumber: 1000000},
	{DID: "bafybmihiwapl23qumkcipzmqr6rkbbyyy2n5jdnxsqrpj7djpdkk3t5i2e", Level: 50001, StartTokenNumber: 2000001, EndTokenNumber: 3000000},
	{DID: "bafybmicbaadhaa76j7jlft6vxfqyzisehbekmnpa7r5g2jwqbn6dikfp4y", Level: 50001, StartTokenNumber: 3000001, EndTokenNumber: 4300000},
}

const (
	NetworkMainnet = "mainnet"
	NetworkTestnet = "testnet"
)

func IsAuthorizedTokenRange(network string, level, tokenNumber int) bool {
	if tokenNumber < rbtTokenNumberMin || tokenNumber > rbtTokenNumberMax {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(network)) {
	case NetworkMainnet:
		return level == mainnetLevel
	case NetworkTestnet:
		return level == testnetLevel
	default:
		return false
	}
}

// Apply only to mint entries (previous_transaction_id == ""); transfers
// don't carry the original minter's DID.
func IsAuthorizedMint(network, did string, level, tokenNumber int) bool {
	var table []MintAccessRange
	switch strings.ToLower(strings.TrimSpace(network)) {
	case NetworkMainnet:
		table = mainnetAllowlist
	case NetworkTestnet:
		table = testnetAllowlist
	default:
		return false
	}
	for _, e := range table {
		if e.DID == did && e.Level == level &&
			tokenNumber >= e.StartTokenNumber && tokenNumber <= e.EndTokenNumber {
			return true
		}
	}
	return false
}

func NormalizeNetwork(network string) string {
	switch strings.ToLower(strings.TrimSpace(network)) {
	case NetworkMainnet:
		return NetworkMainnet
	case NetworkTestnet:
		return NetworkTestnet
	default:
		return ""
	}
}
