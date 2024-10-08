// pkg/genesis/genesis.go

package genesis

import (
	"crypto/sha256"
	"fmt"
	"math/big"
	"os"

	"gopkg.in/yaml.v2"

	"github.com/peerdns/peerdns/pkg/signatures"
	"github.com/peerdns/peerdns/pkg/types"
)

// Genesis represents the genesis block configuration.
type Genesis struct {
	Config     Config               `yaml:"config"`
	Alloc      map[string]AllocItem `yaml:"alloc"`
	Difficulty uint64               `yaml:"difficulty"`
	Timestamp  int64                `yaml:"timestamp"`
	ExtraData  string               `yaml:"extraData"`
}

// Config represents the chain configuration.
type Config struct {
	ChainID uint64 `yaml:"chainId"`
}

// AllocItem represents an account allocation in the genesis block.
type AllocItem struct {
	Balance string `yaml:"balance"`
}

// LoadGenesis loads the genesis configuration from a YAML file.
func LoadGenesis(genesisPath string) (*Genesis, error) {
	data, err := os.ReadFile(genesisPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read genesis file: %w", err)
	}

	var genesis Genesis
	err = yaml.UnmarshalStrict(data, &genesis)
	if err != nil {
		return nil, fmt.Errorf("failed to parse genesis file: %w", err)
	}

	// Validate the loaded genesis configuration
	if err := genesis.Validate(); err != nil {
		return nil, fmt.Errorf("invalid genesis configuration: %w", err)
	}

	return &genesis, nil
}

// Validate ensures that the genesis configuration is valid.
func (g *Genesis) Validate() error {
	if g.Config.ChainID == 0 {
		return fmt.Errorf("chainId must be greater than zero")
	}
	if g.Difficulty == 0 {
		return fmt.Errorf("difficulty must be greater than zero")
	}
	if g.Timestamp == 0 {
		return fmt.Errorf("timestamp must be set")
	}
	if len(g.Alloc) == 0 {
		return fmt.Errorf("alloc must have at least one account")
	}
	for addr, alloc := range g.Alloc {
		if addr == "" {
			return fmt.Errorf("allocation address cannot be empty")
		}
		if alloc.Balance == "" {
			return fmt.Errorf("balance for address %s cannot be empty", addr)
		}
	}
	return nil
}

// CreateGenesisBlock creates the genesis block based on the provided genesis configuration.
// mAddr is the miner/validator address for the genesis block.
func CreateGenesisBlock(mAddr types.Address, genesis *Genesis) (*types.Block, error) {
	genesisTransactions := make([]*types.Transaction, 0, len(genesis.Alloc))

	for addrStr, alloc := range genesis.Alloc {
		address, err := types.FromHex(addrStr)
		if err != nil {
			return nil, fmt.Errorf("invalid address %s: %w", addrStr, err)
		}

		balance, ok := new(big.Int).SetString(alloc.Balance, 10)
		if !ok {
			return nil, fmt.Errorf("invalid balance format for address %s: %s", addrStr, alloc.Balance)
		}

		txID := sha256.Sum256([]byte(fmt.Sprintf("alloc-%s", addrStr)))

		tx := &types.Transaction{
			ID:            txID,
			Sender:        mAddr,
			Recipient:     address,
			Amount:        balance.Uint64(),
			Fee:           0, // No fee for genesis allocations
			Nonce:         0,
			Timestamp:     genesis.Timestamp,
			Payload:       []byte("Initial allocation"),
			Signature:     []byte{},
			SignatureType: signatures.BlsSignerType,
		}
		genesisTransactions = append(genesisTransactions, tx)
	}

	genesisBlock, err := types.NewBlock(
		0,                   // Index
		types.ZeroHash,      // PreviousHash (zeroed)
		genesisTransactions, // Transactions
		mAddr,               // Miner address (genesis sender)
		genesis.Difficulty,  // Difficulty
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create genesis block: %w", err)
	}

	return genesisBlock, nil
}
