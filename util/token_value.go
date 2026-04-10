package util

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// MaxSupportedDecimalPlaces matches rubixgoplatform/constants
const MaxSupportedDecimalPlaces = 3

// TreeLevelRanges holds [min, max] part-index range for each tree level.
var TreeLevelRanges [][2]int

func init() {
	TreeLevelRanges = computeTreeLevelRanges()
}

// getNumberOfChildren returns the branching factor for a node at the given level.
func getNumberOfChildren(parentLevel int) int {
	if parentLevel%2 == 0 {
		return 2
	}
	return 5
}

// computeTreeLevelRanges dynamically builds the [min, max] part-index range
// for each level (0-6) of the token-subdivision tree.
func computeTreeLevelRanges() [][2]int {
	scaledValue := 1
	for i := 0; i < MaxSupportedDecimalPlaces; i++ {
		scaledValue *= 10
	}
	smallestUnit := 1

	divisors := []int{}
	current := scaledValue
	level := 0
	for current > smallestUnit {
		d := getNumberOfChildren(level)
		divisors = append(divisors, d)
		current /= d
		level++
	}
	maxDepth := len(divisors)

	ranges := make([][2]int, maxDepth+1)
	ranges[0] = [2]int{0, 0}

	nodeCount := 1
	nextMin := 1

	for l := 1; l <= maxDepth; l++ {
		nodeCount *= divisors[l-1]
		min := nextMin
		max := min + nodeCount - 1
		ranges[l] = [2]int{min, max}
		nextMin = max + 1
	}

	return ranges
}

// RbtIDElements holds the parsed components of an RBT token ID.
type RbtIDElements struct {
	Level       int
	TokenNumber int
	PartIndex   int
}

// GetRbtIDElements parses an RBT token ID string (e.g. "x_yzabcdef" or "x_yzabcdef_ghijkl")
// into its constituent elements: network level, token number, and optional part index.
func GetRbtIDElements(tokenID string) (RbtIDElements, error) {
	// check if token id is ft id, by checking if the length of the id is more than length of DID (59)
	if len(tokenID) > 59 {
		return RbtIDElements{}, fmt.Errorf("invalid token id format for rbt: %s, id length should be <= 59", tokenID)
	}

	idElems := strings.Split(tokenID, "_")
	if len(idElems) < 2 || len(idElems) > 3 { // ensure id is in proper RBT id format
		return RbtIDElements{}, fmt.Errorf("invalid token id format for rbt: %s, id elements should be 2 (whole) or 3 (part)", tokenID)
	}

	var err error
	rbtElems := RbtIDElements{}

	rbtElems.Level, err = strconv.Atoi(idElems[0])
	if err != nil {
		return RbtIDElements{}, fmt.Errorf("failed to convert level into int for rbt: %s, error: %v", tokenID, err)
	}
	rbtElems.TokenNumber, err = strconv.Atoi(idElems[1])
	if err != nil {
		return RbtIDElements{}, fmt.Errorf("failed to convert token number into int for rbt: %s, error: %v", tokenID, err)
	}

	switch len(idElems) {
	case 2:
		rbtElems.PartIndex = 0 // Case for whole token
	case 3:
		rbtElems.PartIndex, err = strconv.Atoi(idElems[2]) // Case for part token
		if err != nil {
			return RbtIDElements{}, fmt.Errorf("failed to convert part index into int for rbt: %s, error: %v", tokenID, err)
		}
	default:
		return RbtIDElements{}, fmt.Errorf("invalid token id format for rbt: %s, id elements should be 2 (whole) or 3 (part)", tokenID)
	}
	return rbtElems, nil
}

// GetTreeLevelFromPartIndex returns the tree level (0-6) for a given part index x.
func GetTreeLevelFromPartIndex(x int) (int, error) {
	if x == 0 {
		return 0, nil // Level 0 is the root (Whole token)
	}
	for level, r := range TreeLevelRanges {
		if x >= r[0] && x <= r[1] {
			return level, nil
		}
	}
	return 0, fmt.Errorf("part index %d is out of range (valid: 1 - 1332)", x)
}

// LevelToDenom converts a tree level to its denomination value.
// Level 0 -> 1.0 (Whole Token)
func LevelToDenom(level int) (float64, error) {
	if level < 0 {
		return 0, fmt.Errorf("LevelToDenom: level cannot be negative, provided level: %v", level)
	}

	k := level / 2
	if level%2 == 0 {
		return math.Pow(10, -float64(k)), nil
	}
	return 5 * math.Pow(10, -float64(k+1)), nil
}

// GetTokenValueFromTokenID derives the token's denomination value directly
// from its RBT token ID format (e.g. "x_yzabcdef" -> 1.0, "x_yzabcdef_ghijkl" -> derived value).
func GetTokenValueFromTokenID(tokenID string) (float64, error) {
	// split RBT id elements
	tokenElems, err := GetRbtIDElements(tokenID)
	if err != nil {
		return 0, fmt.Errorf("GetTokenValueFromTokenID: failed to split elements of token ID %s, err: %v", tokenID, err)
	}

	// get token level
	tokenTreeLevel, err := GetTreeLevelFromPartIndex(tokenElems.PartIndex)
	if err != nil {
		return 0, fmt.Errorf("GetTokenValueFromTokenID: failed to get tree level for token %s, err: %v", tokenID, err)
	}

	// get token value from tree level
	return LevelToDenom(tokenTreeLevel)
}
