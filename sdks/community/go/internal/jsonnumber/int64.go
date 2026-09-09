// Package jsonnumber converts exact JSON integers without floating-point loss.
package jsonnumber

import (
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"
	"strings"
)

// Int64 accepts one JSON numeric token representing an exact int64 integer.
// Decimal and exponent spellings are valid; nonnumeric JSON and fractions are not.
func Int64(raw []byte) (int64, error) {
	s := string(raw)
	if len(s) == 0 || (s[0] != '-' && (s[0] < '0' || s[0] > '9')) || !json.Valid(raw) {
		return 0, fmt.Errorf("must be a JSON number")
	}
	// Bound exponent magnitude before exact arithmetic so allocation is bounded
	// by input length, even for values such as 1e-999999999.
	mantissa, exponent := s, "0"
	if i := strings.IndexAny(s, "eE"); i >= 0 {
		mantissa, exponent = s[:i], s[i+1:]
	}
	if strings.Trim(mantissa, "-0.") == "" {
		return 0, nil
	}
	power, err := strconv.ParseInt(exponent, 10, 64)
	bound := int64(len(mantissa)) + 20
	if err != nil || power < -bound || power > bound {
		return 0, fmt.Errorf("must be an exact int64 integer")
	}
	exact, ok := new(big.Rat).SetString(s)
	if !ok || !exact.IsInt() || !exact.Num().IsInt64() {
		return 0, fmt.Errorf("must be an exact int64 integer")
	}
	return exact.Num().Int64(), nil
}
