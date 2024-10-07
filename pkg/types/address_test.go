package types

import (
	"encoding/hex"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddressCreation(t *testing.T) {
	validBytes := make([]byte, AddressLength)
	for i := 0; i < AddressLength; i++ {
		validBytes[i] = byte(i)
	}
	invalidBytes := make([]byte, AddressLength-1)

	t.Run("NewAddress with valid bytes", func(t *testing.T) {
		addr, err := NewAddress(validBytes)
		require.NoError(t, err)
		assert.Equal(t, validBytes, addr.Bytes())
	})

	t.Run("NewAddress with invalid bytes", func(t *testing.T) {
		_, err := NewAddress(invalidBytes)
		assert.Error(t, err)
		assert.Equal(t, ErrInvalidAddressLength, err)
	})

	t.Run("FromBytes with valid bytes", func(t *testing.T) {
		addr := FromBytes(validBytes)
		assert.Equal(t, validBytes, addr.Bytes())
	})

	t.Run("FromBytes with invalid bytes", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Errorf("FromBytes did not panic with invalid input")
			}
		}()
		FromBytes(invalidBytes)
	})

	t.Run("FromHex with valid hex", func(t *testing.T) {
		hexStr := "0x" + hex.EncodeToString(validBytes)
		addr, err := FromHex(hexStr)
		require.NoError(t, err)
		assert.Equal(t, validBytes, addr.Bytes())
	})

	t.Run("FromHex with invalid length", func(t *testing.T) {
		hexStr := "0x1234" // too short
		_, err := FromHex(hexStr)
		assert.Error(t, err)
		assert.Equal(t, ErrInvalidAddressLength, err)
	})

	t.Run("FromHex with invalid hex", func(t *testing.T) {
		hexStr := "0xZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZ"
		_, err := FromHex(hexStr)
		assert.Error(t, err)
	})

	t.Run("MustFromHex with valid hex", func(t *testing.T) {
		hexStr := "0x" + hex.EncodeToString(validBytes)
		addr := MustFromHex(hexStr)
		assert.Equal(t, validBytes, addr.Bytes())
	})

	t.Run("MustFromHex with invalid hex", func(t *testing.T) {
		hexStr := "0x1234" // too short
		defer func() {
			if r := recover(); r == nil {
				t.Errorf("MustFromHex did not panic with invalid input")
			}
		}()
		MustFromHex(hexStr)
	})
}

func TestAddressMethods(t *testing.T) {
	bytes := []byte{0xde, 0xad, 0xbe, 0xef, 0xca, 0xfe, 0xba, 0xbe, 0x00, 0x11,
		0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88, 0x99, 0xaa, 0xbb}
	addr, err := NewAddress(bytes)
	require.NoError(t, err)

	t.Run("Bytes method", func(t *testing.T) {
		assert.Equal(t, bytes, addr.Bytes())
	})

	t.Run("Hex method", func(t *testing.T) {
		expectedHex := "0xdeadbeefcafebabe00112233445566778899aabb"
		assert.Equal(t, expectedHex, addr.Hex())
	})

	t.Run("String method", func(t *testing.T) {
		expectedStr := "0xdeadbeefcafebabe00112233445566778899aabb"
		assert.Equal(t, expectedStr, addr.String())
	})

	t.Run("Equals method", func(t *testing.T) {
		addrCopy, err := NewAddress(bytes)
		require.NoError(t, err)
		assert.True(t, addr.Equals(addrCopy))

		// Modify one byte
		modifiedBytes := make([]byte, AddressLength)
		copy(modifiedBytes, bytes)
		modifiedBytes[0] = 0xff
		addrModified, err := NewAddress(modifiedBytes)
		require.NoError(t, err)
		assert.False(t, addr.Equals(addrModified))
	})

	t.Run("IsZero method", func(t *testing.T) {
		zeroAddr, err := NewAddress(make([]byte, AddressLength))
		require.NoError(t, err)
		assert.True(t, zeroAddr.IsZero())
		assert.False(t, addr.IsZero())
	})

	t.Run("ShortHex method", func(t *testing.T) {
		expectedShort := "0xdead...aabb"
		assert.Equal(t, expectedShort, addr.ShortHex())
	})

	t.Run("MarshalText", func(t *testing.T) {
		text, err := addr.MarshalText()
		require.NoError(t, err)
		expected := "0xdeadbeefcafebabe00112233445566778899aabb"
		assert.Equal(t, []byte(expected), text)
	})

	t.Run("UnmarshalText", func(t *testing.T) {
		var newAddr Address
		text := "0xdeadbeefcafebabe00112233445566778899aabb"
		err := newAddr.UnmarshalText([]byte(text))
		require.NoError(t, err)
		assert.True(t, addr.Equals(newAddr))
	})

	t.Run("MarshalJSON", func(t *testing.T) {
		jsonBytes, err := addr.MarshalJSON()
		require.NoError(t, err)
		expectedJSON := `"0xdeadbeefcafebabe00112233445566778899aabb"`
		assert.Equal(t, expectedJSON, string(jsonBytes))
	})

	t.Run("UnmarshalJSON", func(t *testing.T) {
		var newAddr Address
		jsonStr := `"0xdeadbeefcafebabe00112233445566778899aabb"`
		err := newAddr.UnmarshalJSON([]byte(jsonStr))
		require.NoError(t, err)
		assert.True(t, addr.Equals(newAddr))
	})

	t.Run("MarshalText with invalid address", func(t *testing.T) {
		var invalidAddr Address
		invalidAddr = FromBytes(make([]byte, AddressLength))
		// Assuming empty address is allowed in MarshalText
		text, err := invalidAddr.MarshalText()
		require.NoError(t, err)
		expected := "0x0000000000000000000000000000000000000000"
		assert.Equal(t, []byte(expected), text)
	})
}

func TestAddressJSONMarshalling(t *testing.T) {
	bytes := []byte{0x12, 0x34, 0x56, 0x78, 0x9a, 0xbc, 0xde, 0xf0, 0x11, 0x22,
		0x33, 0x44, 0x55, 0x66, 0x77, 0x88, 0x99, 0xaa, 0xbb, 0xcc}
	addr, err := NewAddress(bytes)
	require.NoError(t, err)

	t.Run("Marshal to JSON", func(t *testing.T) {
		data, err := json.Marshal(addr)
		require.NoError(t, err)
		expected := `"0x123456789abcdef0112233445566778899aabbcc"`
		assert.JSONEq(t, expected, string(data))
	})

	t.Run("Unmarshal from JSON", func(t *testing.T) {
		jsonStr := `"0x123456789abcdef0112233445566778899aabbcc"`
		var newAddr Address
		err := json.Unmarshal([]byte(jsonStr), &newAddr)
		require.NoError(t, err)
		assert.True(t, addr.Equals(newAddr))
	})

	t.Run("Unmarshal invalid JSON", func(t *testing.T) {
		jsonStr := `"0x1234"` // Too short
		var newAddr Address
		err := json.Unmarshal([]byte(jsonStr), &newAddr)
		assert.Error(t, err)
	})

	t.Run("Marshal and Unmarshal consistency", func(t *testing.T) {
		data, err := json.Marshal(addr)
		require.NoError(t, err)

		var unmarshaledAddr Address
		err = json.Unmarshal(data, &unmarshaledAddr)
		require.NoError(t, err)

		assert.True(t, addr.Equals(unmarshaledAddr))
	})
}
