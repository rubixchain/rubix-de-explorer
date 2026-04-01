package util

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// MaxSupportedDecimalPlaces matches rubixgoplatform/constants
const MaxSupportedDecimalPlaces = 3

// treeLevelRanges holds [min, max] part-index range for each tree level.
// Computed once at init time.
var treeLevelRanges [][2]int

func init() {
	treeLevelRanges = computeTreeLevelRanges()
}

// getNumberOfChildren returns the branching factor for a node at the given level.
// Even levels split into 2 children, odd levels split into 5 children.
func getNumberOfChildren(parentLevel int) int {
	if parentLevel%2 == 0 {
		return 2
	}
	return 5
}

// computeTreeLevelRanges dynamically builds the [min, max] part-index range
// for each level of the token-subdivision tree.
//
// The tree subdivides 1 token down to MaxSupportedDecimalPlaces decimal places:
//   - Even-level nodes have 2 children (÷2 subdivision)
//   - Odd-level nodes have 5 children (÷5 subdivision)
//
// With MaxSupportedDecimalPlaces=3:
//
//	L0=1.0(root) ; L1=0.5(÷2) ; L2=0.1(÷5) ; L3=0.05(÷2) ;
//	L4=0.01(÷5) ; L5=0.005(÷2) ; L6=0.001(÷5) ; maxDepth=6
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

// getTreeLevelFromPartIndex returns the tree level for a given part index.
func getTreeLevelFromPartIndex(x int) (int, error) {
	for level, r := range treeLevelRanges {
		if x >= r[0] && x <= r[1] {
			return level, nil
		}
	}
	return 0, fmt.Errorf("part index %d is out of range", x)
}

// LevelToDenom converts a tree level to its denomination value.
//
//	Level 0 -> 1.0   (whole token)
//	Level 1 -> 0.5
//	Level 2 -> 0.1
//	Level 3 -> 0.05
//	Level 4 -> 0.01
//	Level 5 -> 0.005
//	Level 6 -> 0.001
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

// RbtIDElements holds the parsed components of an RBT token ID.
type RbtIDElements struct {
	Level       int
	TokenNumber int
	PartIndex   int
}

// ParseRbtTokenID parses an RBT token ID string (e.g. "1_12345" or "1_12345_3")
// into its constituent elements.
func ParseRbtTokenID(tokenID string) (RbtIDElements, error) {
	elems := strings.Split(tokenID, "_")
	if len(elems) < 2 || len(elems) > 3 {
		return RbtIDElements{}, fmt.Errorf("invalid RBT token ID format: %s, expected 2 or 3 parts separated by '_'", tokenID)
	}

	level, err := strconv.Atoi(elems[0])
	if err != nil {
		return RbtIDElements{}, fmt.Errorf("failed to parse level from RBT token ID: %s, error: %v", tokenID, err)
	}

	tokenNumber, err := strconv.Atoi(elems[1])
	if err != nil {
		return RbtIDElements{}, fmt.Errorf("failed to parse token number from RBT token ID: %s, error: %v", tokenID, err)
	}

	result := RbtIDElements{
		Level:       level,
		TokenNumber: tokenNumber,
		PartIndex:   0, // whole token default
	}

	if len(elems) == 3 {
		result.PartIndex, err = strconv.Atoi(elems[2])
		if err != nil {
			return RbtIDElements{}, fmt.Errorf("failed to parse part index from RBT token ID: %s, error: %v", tokenID, err)
		}
	}

	return result, nil
}

// GetTokenValueFromTokenID derives the token's denomination value directly
// from its RBT token ID format (e.g. "1_12345" -> 1.0, "1_12345_3" -> 0.1).
//
// The algorithm:
//  1. Parse the token ID to extract the PartIndex
//  2. Map the PartIndex to a tree level using pre-computed ranges
//  3. Convert the tree level to a denomination value
func GetTokenValueFromTokenID(tokenID string) (float64, error) {
	tokenElems, err := ParseRbtTokenID(tokenID)
	if err != nil {
		return 0, fmt.Errorf("GetTokenValueFromTokenID: %v", err)
	}

	treeLevel, err := getTreeLevelFromPartIndex(tokenElems.PartIndex)
	if err != nil {
		return 0, fmt.Errorf("GetTokenValueFromTokenID: failed to get tree level for token %s, err: %v", tokenID, err)
	}

	return LevelToDenom(treeLevel)
}
