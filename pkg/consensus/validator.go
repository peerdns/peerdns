// pkg/consensus/validator.go
package consensus

import (
	"log"
	"sync"

	"github.com/libp2p/go-libp2p/core/peer"
)

// Validator represents an individual validator participating in the consensus.
type Validator struct {
	ID         peer.ID        // Validator's peer ID
	PublicKey  *BLSPublicKey  // Validator's BLS public key
	PrivateKey *BLSPrivateKey // Validator's BLS private key (optional, only for leader)
}

// ValidatorSet manages the collection of validators participating in consensus.
type ValidatorSet struct {
	validators      map[peer.ID]*Validator // Map of validators indexed by peer ID
	leaderID        peer.ID                // Current leader's peer ID
	mutex           sync.RWMutex           // Mutex for safe access
	quorumThreshold int                    // Quorum threshold for block finalization
	logger          *log.Logger            // Logger for tracking validator activities
}

// NewValidatorSet creates a new set of validators.
func NewValidatorSet(logger *log.Logger) *ValidatorSet {
	return &ValidatorSet{
		validators: make(map[peer.ID]*Validator),
		logger:     logger,
	}
}

// AddValidator adds a new validator to the set.
func (vs *ValidatorSet) AddValidator(id peer.ID, publicKey *BLSPublicKey) {
	vs.mutex.Lock()
	defer vs.mutex.Unlock()
	vs.validators[id] = &Validator{ID: id, PublicKey: publicKey}
	vs.logger.Printf("Added validator: %s", id)
}

// RemoveValidator removes a validator from the set.
func (vs *ValidatorSet) RemoveValidator(id peer.ID) {
	vs.mutex.Lock()
	defer vs.mutex.Unlock()
	delete(vs.validators, id)
	vs.logger.Printf("Removed validator: %s", id)
}

// GetValidator retrieves a validator by peer ID.
func (vs *ValidatorSet) GetValidator(id peer.ID) *Validator {
	vs.mutex.RLock()
	defer vs.mutex.RUnlock()
	return vs.validators[id]
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

// ElectLeader randomly selects a leader for the current round.
func (vs *ValidatorSet) ElectLeader() {
	vs.mutex.Lock()
	defer vs.mutex.Unlock()
	for id := range vs.validators {
		vs.leaderID = id // Select the first available validator as the leader (simplified)
		break
	}
	vs.logger.Printf("Elected leader: %s", vs.leaderID)
}

// CurrentLeader returns the current leader's peer ID.
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
	vs.logger.Printf("Set quorum threshold: %d", threshold)
}
