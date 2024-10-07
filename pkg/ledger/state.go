package ledger

import (
	"sync"

	"github.com/peerdns/peerdns/pkg/types"
)

// State represents the ledger's state, including balances and nonces for addresses.
type State struct {
	balances map[string]uint64 // Map from address hex string to balance
	nonces   map[string]uint64 // Map from address hex string to nonce
	mutex    sync.RWMutex
}

// NewState creates a new State instance.
func NewState() *State {
	return &State{
		balances: make(map[string]uint64),
		nonces:   make(map[string]uint64),
	}
}

// GetBalance retrieves the balance of an address.
func (s *State) GetBalance(address types.Address) uint64 {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.balances[address.Hex()]
}

// UpdateBalance updates the balance of an address by the given amount.
// The amount can be negative (for deductions) or positive (for additions).
func (s *State) UpdateBalance(address types.Address, amount int64) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	addrStr := address.Hex()
	currentBalance := s.balances[addrStr]
	newBalance := int64(currentBalance) + amount
	if newBalance < 0 {
		newBalance = 0 // Prevent negative balances
	}
	s.balances[addrStr] = uint64(newBalance)
}

// GetNonce retrieves the nonce of an address.
func (s *State) GetNonce(address types.Address) uint64 {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.nonces[address.Hex()]
}

// IncrementNonce increments the nonce of an address.
func (s *State) IncrementNonce(address types.Address) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	addrStr := address.Hex()
	s.nonces[addrStr]++
}

// applyBlock applies the transactions in the block to the ledger's state.
func (l *Ledger) applyBlock(block *types.Block) error {
	for _, tx := range block.Transactions {
		// Deduct amount and fee from sender
		l.state.UpdateBalance(tx.Sender, -int64(tx.Amount+tx.Fee))
		// Increment sender's nonce
		l.state.IncrementNonce(tx.Sender)
		// Add amount to recipient
		l.state.UpdateBalance(tx.Recipient, int64(tx.Amount))
		// Add fee to miner/validator (assuming ValidatorID is the miner)
		l.state.UpdateBalance(block.ValidatorID, int64(tx.Fee))
	}
	return nil
}
