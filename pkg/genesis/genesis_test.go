// pkg/genesis/genesis_test.go

package genesis

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/peerdns/peerdns/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper function to create a types.Address from a string by hashing and taking the first 20 bytes.
func createAddress(id string) types.Address {
	// Hash the input string to get a consistent length output
	hash := sha256.Sum256([]byte(id))
	// Take the first 20 bytes of the hash to create the address
	var addrBytes types.Address
	copy(addrBytes[:], hash[:types.AddressSize])
	return addrBytes
}

// Helper function to create a temporary genesis YAML file for testing.
func createTempGenesisFile(t *testing.T, content string) string {
	tmpFile, err := os.CreateTemp("", "genesis_*.yaml")
	require.NoError(t, err, "Failed to create temporary genesis file")

	_, err = tmpFile.Write([]byte(content))
	require.NoError(t, err, "Failed to write to temporary genesis file")

	err = tmpFile.Close()
	require.NoError(t, err, "Failed to close temporary genesis file")

	return tmpFile.Name()
}

// TestLoadGenesis tests the LoadGenesis function.
func TestLoadGenesis(t *testing.T) {
	validGenesisYAML := `
config:
  chainId: 1
alloc:
  "0000000000000000000000000000000000000001":
    balance: "1000000"
  "0000000000000000000000000000000000000002":
    balance: "2000000"
difficulty: 1
timestamp: ` + fmt.Sprintf("%d", time.Now().Unix()) + `
extraData: "Genesis Block"
`

	invalidGenesisYAML := `
config:
  chainId: 0
alloc:
  "0000000000000000000000000000000000000001":
    balance: "1000000"
difficulty: 0
timestamp: 0
extraData: "Invalid Genesis Block"
`

	t.Run("LoadValidGenesis", func(t *testing.T) {
		genesisPath := createTempGenesisFile(t, validGenesisYAML)
		defer os.Remove(genesisPath)

		genesis, err := LoadGenesis(genesisPath)
		require.NoError(t, err, "Failed to load valid genesis")

		assert.Equal(t, uint64(1), genesis.Config.ChainID, "ChainID should match")
		assert.Equal(t, uint64(1), genesis.Difficulty, "Difficulty should match")
		assert.NotZero(t, genesis.Timestamp, "Timestamp should be set")
		assert.Equal(t, "Genesis Block", genesis.ExtraData, "ExtraData should match")
		assert.Len(t, genesis.Alloc, 2, "Alloc should have two entries")
	})

	t.Run("LoadInvalidGenesis", func(t *testing.T) {
		genesisPath := createTempGenesisFile(t, invalidGenesisYAML)
		defer os.Remove(genesisPath)

		genesis, err := LoadGenesis(genesisPath)
		require.Error(t, err, "Expected error when loading invalid genesis")
		assert.Nil(t, genesis, "Genesis should be nil on failure")
	})

	t.Run("LoadMalformedYAML", func(t *testing.T) {
		malformedYAML := `
config:
  chainId: 1
  homesteadBlock: 0
  eip150Block: 0
  eip155Block: 0
  eip158Block: 0
  byzantiumBlock: 0
  constantinopleBlock: 0
  petersburgBlock: 0
alloc:
  "0000000000000000000000000000000000000001":
    balance: "1000000"
difficulty: 1
timestamp: not_a_timestamp
extraData: "Genesis Block"
`
		genesisPath := createTempGenesisFile(t, malformedYAML)
		defer os.Remove(genesisPath)

		genesis, err := LoadGenesis(genesisPath)
		require.Error(t, err, "Expected error when loading malformed YAML")
		assert.Nil(t, genesis, "Genesis should be nil on failure")
	})
}

// TestCreateGenesisBlock tests the CreateGenesisBlock function.
func TestCreateGenesisBlock(t *testing.T) {
	genesisConfig := &Genesis{
		Config: Config{
			ChainID: 1,
		},
		Alloc: map[string]AllocItem{
			"0000000000000000000000000000000000000001": {Balance: "1000000"},
			"0000000000000000000000000000000000000002": {Balance: "2000000"},
		},
		Difficulty: 1,
		Timestamp:  time.Now().Unix(),
		ExtraData:  "Genesis Block",
	}

	minerAddress := createAddress("did:peer:miner1") // Example address

	genesisBlock, err := CreateGenesisBlock(minerAddress, genesisConfig)
	require.NoError(t, err, "Failed to create genesis block")

	assert.Equal(t, uint64(0), genesisBlock.Index, "Genesis block index should be 0")
	assert.True(t, types.HashEqual(genesisBlock.PreviousHash, types.ZeroHash), "Genesis block should have zero previous hash")
	assert.Equal(t, minerAddress, genesisBlock.ValidatorID, "ValidatorID should match miner address")
	assert.Equal(t, genesisConfig.Difficulty, genesisBlock.Difficulty, "Difficulty should match")
	assert.Equal(t, genesisConfig.Timestamp, genesisBlock.Timestamp, "Timestamp should match")
	assert.Len(t, genesisBlock.Transactions, 2, "Genesis block should have two transactions")

	for _, tx := range genesisBlock.Transactions {
		assert.True(t, tx.Sender.IsZero(), "Transaction sender should be zero address")
		assert.True(t, tx.Fee == 0, "Transaction fee should be zero")
		assert.True(t, tx.Amount > 0, "Transaction amount should be positive")
		assert.Equal(t, "Initial allocation", string(tx.Payload), "Transaction payload should match")
	}
}

// TestCreateGenesisBlock_InvalidAlloc tests CreateGenesisBlock with invalid allocations.
func TestCreateGenesisBlock_InvalidAlloc(t *testing.T) {
	genesisConfig := &Genesis{
		Config: Config{
			ChainID: 1,
		},
		Alloc: map[string]AllocItem{
			"":                       {Balance: "1000000"}, // Empty address
			"invalid_address_format": {Balance: "1000"},    // Invalid address
			"0000000000000000000000000000000000000003": {Balance: "not_a_number"}, // Invalid balance
		},
		Difficulty: 1,
		Timestamp:  time.Now().Unix(),
		ExtraData:  "Genesis Block",
	}

	minerAddress := createAddress("did:peer:miner1") // Example address

	genesisBlock, err := CreateGenesisBlock(minerAddress, genesisConfig)
	require.Error(t, err, "Expected error when creating genesis block with invalid allocations")
	assert.Nil(t, genesisBlock, "Genesis block should be nil on failure")
}

// TestValidateGenesis tests the Validate method of the Genesis struct.
func TestValidateGenesis(t *testing.T) {
	validGenesis := &Genesis{
		Config: Config{
			ChainID: 1,
		},
		Alloc: map[string]AllocItem{
			"0000000000000000000000000000000000000001": {Balance: "1000000"},
		},
		Difficulty: 1,
		Timestamp:  time.Now().Unix(),
		ExtraData:  "Valid Genesis",
	}

	err := validGenesis.Validate()
	require.NoError(t, err, "Valid genesis should pass validation")

	invalidGenesis := &Genesis{
		Config: Config{
			ChainID: 0, // Invalid ChainID
		},
		Alloc: map[string]AllocItem{
			"": {Balance: "1000000"}, // Empty address
		},
		Difficulty: 0, // Invalid Difficulty
		Timestamp:  0, // Invalid Timestamp
		ExtraData:  "",
	}

	err = invalidGenesis.Validate()
	require.Error(t, err, "Invalid genesis should fail validation")
}

// TestLoadGenesis_FileNotFound tests loading a genesis file that does not exist.
func TestLoadGenesis_FileNotFound(t *testing.T) {
	genesisPath := filepath.Join(os.TempDir(), "non_existent_genesis.yaml")
	genesis, err := LoadGenesis(genesisPath)
	require.Error(t, err, "Expected error when loading non-existent genesis file")
	assert.Nil(t, genesis, "Genesis should be nil on failure")
}

// TestCreateGenesisBlock_ZeroAddress tests creating a genesis block with a zero miner address.
func TestCreateGenesisBlock_ZeroAddress(t *testing.T) {
	genesisConfig := &Genesis{
		Config: Config{
			ChainID: 1,
		},
		Alloc: map[string]AllocItem{
			"0000000000000000000000000000000000000001": {Balance: "1000000"},
		},
		Difficulty: 1,
		Timestamp:  time.Now().Unix(),
		ExtraData:  "Genesis Block",
	}

	minerAddress := types.ZeroAddress // Zero address

	genesisBlock, err := CreateGenesisBlock(minerAddress, genesisConfig)
	require.NoError(t, err, "Failed to create genesis block with zero miner address")

	assert.Equal(t, types.ZeroAddress, genesisBlock.ValidatorID, "ValidatorID should be zero address")
}

// TestCreateGenesisBlock_ExtraFields tests that extra fields in the genesis configuration are handled correctly.
func TestCreateGenesisBlock_ExtraFields(t *testing.T) {
	genesisConfig := &Genesis{
		Config: Config{
			ChainID: 1,
		},
		Alloc: map[string]AllocItem{
			"0000000000000000000000000000000000000001": {Balance: "1000000"},
		},
		Difficulty: 1,
		Timestamp:  time.Now().Unix(),
		ExtraData:  "Genesis Block with Extra Fields",
	}

	minerAddress := createAddress("did:peer:miner1")

	genesisBlock, err := CreateGenesisBlock(minerAddress, genesisConfig)
	require.NoError(t, err, "Failed to create genesis block with extra fields")
	require.NotNil(t, genesisBlock, "Genesis block should not be nil")
}

// TestCreateGenesisBlock_MultipleAllocations tests creating a genesis block with multiple allocations.
func TestCreateGenesisBlock_MultipleAllocations(t *testing.T) {
	genesisConfig := &Genesis{
		Config: Config{
			ChainID: 1,
		},
		Alloc: map[string]AllocItem{
			"0000000000000000000000000000000000000001": {Balance: "1000000"},
			"0000000000000000000000000000000000000002": {Balance: "2000000"},
			"0000000000000000000000000000000000000003": {Balance: "3000000"},
		},
		Difficulty: 1,
		Timestamp:  time.Now().Unix(),
		ExtraData:  "Genesis Block with Multiple Allocations",
	}

	minerAddress := createAddress("did:peer:miner1")

	genesisBlock, err := CreateGenesisBlock(minerAddress, genesisConfig)
	require.NoError(t, err, "Failed to create genesis block with multiple allocations")
	require.NotNil(t, genesisBlock, "Genesis block should not be nil")

	assert.Len(t, genesisBlock.Transactions, 3, "Genesis block should have three transactions")

	for addrStr, alloc := range genesisConfig.Alloc {
		address, err := types.FromHex(addrStr)
		require.NoError(t, err, "Failed to parse allocation address %s", addrStr)

		found := false
		for _, tx := range genesisBlock.Transactions {
			if tx.Recipient.Equals(address) {
				assert.Equal(t, alloc.Balance, fmt.Sprintf("%d", tx.Amount), "Transaction amount should match allocation for address %s", addrStr)
				found = true
				break
			}
		}
		assert.True(t, found, "Allocation for address %s should be present in genesis transactions", addrStr)
	}
}
