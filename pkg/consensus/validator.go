// pkg/consensus/validator_set.go
package consensus

import (
	"sort"
	"sync"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/peerdns/peerdns/pkg/encryption"
	"github.com/peerdns/peerdns/pkg/logger"
)

// Validator represents an individual validator participating in the consensus.
type Validator struct {
	ID         peer.ID                   // Validator's peer ID
	PublicKey  *encryption.BLSPublicKey  // Validator's BLS public key
	PrivateKey *encryption.BLSPrivateKey // Validator's BLS private key (optional, only for leader)
}

// ValidatorSet manages the collection of validators participating in consensus.
type ValidatorSet struct {
	validators      map[peer.ID]*Validator // Map of validators indexed by peer ID
	leaderID        peer.ID                // Current leader's peer ID
	mutex           sync.RWMutex           // Mutex for safe access
	quorumThreshold int                    // Quorum threshold for block finalization
	logger          logger.Logger          // Using logger.Logger interface
}

// NewValidatorSet creates a new set of validators.
func NewValidatorSet(logger logger.Logger) *ValidatorSet {
	return &ValidatorSet{
		validators: make(map[peer.ID]*Validator),
		logger:     logger,
	}
}

// AddValidator adds a new validator to the set.
func (vs *ValidatorSet) AddValidator(id peer.ID, publicKey *encryption.BLSPublicKey, privateKey *encryption.BLSPrivateKey) {
	vs.mutex.Lock()
	defer vs.mutex.Unlock()
	vs.validators[id] = &Validator{ID: id, PublicKey: publicKey, PrivateKey: privateKey}
	vs.logger.Debug("Added validator", "validatorID", id.String())
}

// RemoveValidator removes a validator from the set.
func (vs *ValidatorSet) RemoveValidator(id peer.ID) {
	vs.mutex.Lock()
	defer vs.mutex.Unlock()
	delete(vs.validators, id)
	vs.logger.Debug("Removed validator", "validatorID", id.String())
}

// GetValidator retrieves a validator by peer ID.
func (vs *ValidatorSet) GetValidator(id peer.ID) *Validator {
	vs.mutex.RLock()
	defer vs.mutex.RUnlock()
	if validator, exists := vs.validators[id]; exists {
		return validator
	}
	return nil
}

// GetAllValidators returns a list of all validators.
func (vs *ValidatorSet) GetAllValidators() []*Validator {
	vs.mutex.RLock()
	defer vs.mutex.RUnlock()
	validators := make([]*Validator, 0, len(vs.validators))
	for _, v := range vs.validators {
		validators = append(validators, v)
	}
	return validators
}

// ElectLeader selects a leader for the current round deterministically.
func (vs *ValidatorSet) ElectLeader() {
	vs.mutex.Lock()
	defer vs.mutex.Unlock()

	if len(vs.validators) == 0 {
		vs.logger.Warn("No validators to elect leader from")
		return
	}

	// Collect all validator IDs into a slice
	validatorIDs := make([]peer.ID, 0, len(vs.validators))
	for id := range vs.validators {
		validatorIDs = append(validatorIDs, id)
	}

	// Sort the slice to have deterministic leader selection
	sort.Slice(validatorIDs, func(i, j int) bool {
		return string(validatorIDs[i]) < string(validatorIDs[j])
	})

	// Select the first validator as the leader
	vs.leaderID = validatorIDs[0]
	vs.logger.Debug("Elected leader", "leaderID", vs.leaderID.String())
}

// CurrentLeader returns the current leader's Validator.
func (vs *ValidatorSet) CurrentLeader() *Validator {
	vs.mutex.RLock()
	defer vs.mutex.RUnlock()
	return vs.validators[vs.leaderID]
}

// IsLeader checks if a given validator is the current leader.
func (vs *ValidatorSet) IsLeader(id peer.ID) bool {
	vs.mutex.RLock()
	defer vs.mutex.RUnlock()
	return vs.leaderID == id
}

// QuorumSize returns the number of validators required for quorum.
func (vs *ValidatorSet) QuorumSize() int {
	vs.mutex.RLock()
	defer vs.mutex.RUnlock()
	return vs.quorumThreshold
}

// SetQuorumThreshold sets the quorum threshold for block finalization.
func (vs *ValidatorSet) SetQuorumThreshold(threshold int) {
	vs.mutex.Lock()
	defer vs.mutex.Unlock()
	vs.quorumThreshold = threshold
	vs.logger.Debug("Set quorum threshold", "threshold", threshold)
}

// IsValidator checks if a given peer ID is a validator.
func (vs *ValidatorSet) IsValidator(id peer.ID) bool {
	vs.mutex.RLock()
	defer vs.mutex.RUnlock()
	_, exists := vs.validators[id]
	return exists
}

// SetLeader manually sets the leader (useful for tests or dynamic leader changes).
func (vs *ValidatorSet) SetLeader(id peer.ID) {
	vs.mutex.Lock()
	defer vs.mutex.Unlock()
	if _, exists := vs.validators[id]; exists {
		vs.leaderID = id
		vs.logger.Debug("Set leader", "leaderID", id.String())
	} else {
		vs.logger.Warn("Attempted to set unknown validator as leader", "validatorID", id.String())
	}
}
