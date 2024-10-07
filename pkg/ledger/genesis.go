// pkg/ledger/genesis.go

package ledger

import (
	"crypto/sha256"
	"fmt"
	"io/ioutil"

	"gopkg.in/yaml.v2"

	"github.com/peerdns/peerdns/pkg/types"
)

// Genesis represents the genesis block configuration.
type Genesis struct {
	Config     GenesisConfig        `yaml:"config"`
	Alloc      map[string]AllocItem `yaml:"alloc"`
	Difficulty uint64               `yaml:"difficulty"`
	Timestamp  int64                `yaml:"timestamp"`
	ExtraData  string               `yaml:"extraData"`
}

// GenesisConfig represents the chain configuration.
type GenesisConfig struct {
	ChainID             uint64 `yaml:"chainId"`
	HomesteadBlock      uint64 `yaml:"homesteadBlock"`
	EIP150Block         uint64 `yaml:"eip150Block"`
	EIP155Block         uint64 `yaml:"eip155Block"`
	EIP158Block         uint64 `yaml:"eip158Block"`
	ByzantiumBlock      uint64 `yaml:"byzantiumBlock"`
	ConstantinopleBlock uint64 `yaml:"constantinopleBlock"`
	PetersburgBlock     uint64 `yaml:"petersburgBlock"`
}

// AllocItem represents an account allocation in the genesis block.
type AllocItem struct {
	Balance string `yaml:"balance"`
}

// LoadGenesis loads the genesis configuration from a YAML file.
func LoadGenesis(genesisPath string) (*Genesis, error) {
	data, err := ioutil.ReadFile(genesisPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read genesis file: %w", err)
	}

	var genesis Genesis
	err = yaml.Unmarshal(data, &genesis)
	if err != nil {
		return nil, fmt.Errorf("failed to parse genesis file: %w", err)
	}

	return &genesis, nil
}

// CreateGenesisBlock generates and returns the genesis block for the ledger.
func CreateGenesisBlock(genesis *Genesis) (*types.Block, error) {
	genesisTransactions := make([]*types.Transaction, 0)

	// Initialize accounts with balances
	for addrStr, alloc := range genesis.Alloc {
		address, err := types.FromHex(addrStr)
		if err != nil {
			return nil, fmt.Errorf("invalid address %s: %w", addrStr, err)
		}

		balance, err := types.StringToBigInt(alloc.Balance)
		if err != nil {
			return nil, fmt.Errorf("invalid balance for address %s: %w", addrStr, err)
		}

		// Create a transaction to allocate balance (in a real system, this might be handled differently)
		txID := sha256.Sum256([]byte(fmt.Sprintf("alloc-%s", addrStr)))
		zeroAddress := types.Address{} // Use types.Address{} instead of [types.AddressSize]byte{}

		tx := &types.Transaction{
			ID:        txID,
			Sender:    zeroAddress,
			Recipient: address,
			Amount:    balance.Uint64(),
			Fee:       0,
			Nonce:     0,
			Timestamp: genesis.Timestamp,
			Payload:   []byte("Initial allocation"),
		}
		genesisTransactions = append(genesisTransactions, tx)
	}

	zeroAddress := types.Address{}

	genesisBlock, err := types.NewBlock(
		0,                      // Index
		[types.HashSize]byte{}, // PreviousHash (zeroed)
		genesisTransactions,    // Transactions
		zeroAddress,            // Miner address (zeroed)
		genesis.Difficulty,     // Difficulty
	)
	if err != nil {
		return nil, err
	}

	return genesisBlock, nil
}
