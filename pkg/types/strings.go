package types

import (
	"fmt"
	"math/big"
)

// StringToBigInt converts a string representation of a number to a big.Int.
func StringToBigInt(numberStr string) (*big.Int, error) {
	balance := new(big.Int)
	_, ok := balance.SetString(numberStr, 10) // Assuming the number is in base 10
	if !ok {
		return nil, fmt.Errorf("failed to parse big integer from string: %s", numberStr)
	}
	return balance, nil
}
