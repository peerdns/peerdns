package accounts

import (
	"crypto/sha256"
	"fmt"
	"github.com/peerdns/peerdns/pkg/types"
)

// computeAddressFromPublicKey computes the address from the Ed25519 public key bytes.
func computeAddressFromPublicKey(publicKeyBytes []byte) (types.Address, error) {
	hash := sha256.Sum256(publicKeyBytes)
	if len(hash) < types.AddressSize {
		return types.Address{}, fmt.Errorf("hash too short to derive address")
	}
	addressBytes := hash[len(hash)-types.AddressSize:]
	return types.FromBytes(addressBytes), nil
}
