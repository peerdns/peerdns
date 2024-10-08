package types

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

// ErrInvalidAddressLength is returned when the address does not have the correct length.
var ErrInvalidAddressLength = errors.New("invalid address length")

// Address represents a fixed-size 20-byte Ethereum-like address.
type Address [AddressSize]byte

var (
	ZeroAddress = Address{}
)

func IsZeroAddress(h Address) bool {
	return bytes.Equal(h[:], nil)
}

// NewAddress creates a new Address from a byte slice.
// Returns an error if the slice is not exactly 20 bytes.
func NewAddress(bytes []byte) (Address, error) {
	var addr Address
	if len(bytes) != AddressSize {
		return addr, ErrInvalidAddressLength
	}
	copy(addr[:], bytes)
	return addr, nil
}

// FromBytes creates an Address from a byte slice.
// Panics if the slice is not exactly 20 bytes.
func FromBytes(bytes []byte) Address {
	if len(bytes) != AddressSize {
		panic(fmt.Sprintf("invalid address length: expected %d bytes, got %d", AddressSize, len(bytes)))
	}
	var addr Address
	copy(addr[:], bytes)
	return addr
}

// FromHex creates an Address from a hex string.
// The hex string may have a "0x" prefix.
// Returns an error if the string is not valid hex or does not represent exactly 20 bytes.
func FromHex(s string) (Address, error) {
	var addr Address
	cleaned := strings.TrimPrefix(s, "0x")
	if len(cleaned) != AddressSize*2 {
		return addr, ErrInvalidAddressLength
	}
	bytes, err := hex.DecodeString(cleaned)
	if err != nil {
		return addr, err
	}
	copy(addr[:], bytes)
	return addr, nil
}

// MustFromHex creates an Address from a hex string.
// Panics if the string is not valid hex or does not represent exactly 20 bytes.
func MustFromHex(s string) Address {
	addr, err := FromHex(s)
	if err != nil {
		panic(err)
	}
	return addr
}

// Bytes returns the byte representation of the Address.
func (a Address) Bytes() []byte {
	bytes := make([]byte, AddressSize)
	copy(bytes, a[:])
	return bytes
}

// Hex returns the hexadecimal string representation of the Address with "0x" prefix.
func (a Address) Hex() string {
	return "0x" + hex.EncodeToString(a[:])
}

// String returns the hexadecimal string representation of the Address with "0x" prefix.
// Implements the fmt.Stringer interface.
func (a Address) String() string {
	return a.Hex()
}

// Equals compares two Addresses for equality.
func (a Address) Equals(other Address) bool {
	for i := 0; i < AddressSize; i++ {
		if a[i] != other[i] {
			return false
		}
	}
	return true
}

// MarshalText implements the encoding.TextMarshaler interface.
func (a Address) MarshalText() ([]byte, error) {
	return []byte(a.Hex()), nil
}

// UnmarshalText implements the encoding.TextUnmarshaler interface.
func (a *Address) UnmarshalText(text []byte) error {
	parsed, err := FromHex(string(text))
	if err != nil {
		return err
	}
	*a = parsed
	return nil
}

// MarshalJSON implements the json.Marshaler interface.
func (a Address) MarshalJSON() ([]byte, error) {
	return []byte(`"` + a.Hex() + `"`), nil
}

// UnmarshalJSON implements the json.Unmarshaler interface.
func (a *Address) UnmarshalJSON(data []byte) error {
	str := strings.Trim(string(data), `"`)
	parsed, err := FromHex(str)
	if err != nil {
		return err
	}
	*a = parsed
	return nil
}

// MarshalBinary implements the encoding.BinaryMarshaler interface.
func (a Address) MarshalBinary() ([]byte, error) {
	return a.Bytes(), nil
}

// UnmarshalBinary implements the encoding.BinaryUnmarshaler interface.
func (a *Address) UnmarshalBinary(data []byte) error {
	if len(data) != AddressSize {
		return ErrInvalidAddressLength
	}
	copy(a[:], data)
	return nil
}

// IsZero checks if the address is the zero address.
func (a Address) IsZero() bool {
	for _, b := range a {
		if b != 0 {
			return false
		}
	}
	return true
}

// ShortHex returns a shortened version of the address for display purposes.
func (a Address) ShortHex() string {
	if len(a.Hex()) < 10 {
		return a.Hex()
	}
	return a.Hex()[:6] + "..." + a.Hex()[len(a.Hex())-4:]
}
