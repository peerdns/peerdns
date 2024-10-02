package ledger

import (
	"encoding/json"
	"github.com/peerdns/peerdns/pkg/storage"
	"github.com/pkg/errors"
)

// StateManager handles the state of accounts, contracts, and other ledger data.
type StateManager struct {
	storage storage.Provider // Storage provider for state data
}

// NewStateManager initializes a new StateManager instance.
func NewStateManager(provider storage.Provider) *StateManager {
	return &StateManager{
		storage: provider,
	}
}

// SetAccountState sets the state of a given account in the state storage.
func (sm *StateManager) SetAccountState(accountID string, state map[string]interface{}) error {
	stateKey := []byte("account:" + accountID)
	stateData, err := json.Marshal(state)
	if err != nil {
		return errors.Wrap(err, "failed to serialize account state")
	}

	// Store the account state in the database
	if err := sm.storage.Set(stateKey, stateData); err != nil {
		return errors.Wrap(err, "failed to store account state in database")
	}

	return nil
}

// GetAccountState retrieves the state of a given account from the database.
func (sm *StateManager) GetAccountState(accountID string) (map[string]interface{}, error) {
	stateKey := []byte("account:" + accountID)
	stateData, err := sm.storage.Get(stateKey)
	if err != nil {
		return nil, errors.Wrap(err, "failed to retrieve account state from database")
	}

	// Deserialize the state data into a map
	var state map[string]interface{}
	if err := json.Unmarshal(stateData, &state); err != nil {
		return nil, errors.Wrap(err, "failed to deserialize account state")
	}

	return state, nil
}

// SetContractState sets the state of a given contract in the state storage.
func (sm *StateManager) SetContractState(contractID string, state map[string]interface{}) error {
	stateKey := []byte("contract:" + contractID)
	stateData, err := json.Marshal(state)
	if err != nil {
		return errors.Wrap(err, "failed to serialize contract state")
	}

	// Store the contract state in the database
	if err := sm.storage.Set(stateKey, stateData); err != nil {
		return errors.Wrap(err, "failed to store contract state in database")
	}

	return nil
}

// GetContractState retrieves the state of a given contract from the database.
func (sm *StateManager) GetContractState(contractID string) (map[string]interface{}, error) {
	stateKey := []byte("contract:" + contractID)
	stateData, err := sm.storage.Get(stateKey)
	if err != nil {
		return nil, errors.Wrap(err, "failed to retrieve contract state from database")
	}

	// Deserialize the state data into a map
	var state map[string]interface{}
	if err := json.Unmarshal(stateData, &state); err != nil {
		return nil, errors.Wrap(err, "failed to deserialize contract state")
	}

	return state, nil
}
