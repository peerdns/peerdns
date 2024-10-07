package consensus

import (
	"github.com/peerdns/peerdns/pkg/accounts"
	"github.com/peerdns/peerdns/pkg/metrics"
	"sync"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/peerdns/peerdns/pkg/logger"
)

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
func (vs *ValidatorSet) AddValidator(account *accounts.Account) {
	vs.mutex.Lock()
	defer vs.mutex.Unlock()
	vs.validators[account.PeerID] = &Validator{account: account}
	vs.logger.Debug("Added validator", "validatorID", account.PeerID.String())
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

// GetValidatorIDs returns a slice of all validator peer IDs.
func (vs *ValidatorSet) GetValidatorIDs() []peer.ID {
	vs.mutex.RLock()
	defer vs.mutex.RUnlock()
	ids := make([]peer.ID, 0, len(vs.validators))
	for id := range vs.validators {
		ids = append(ids, id)
	}
	return ids
}

// ElectLeader selects a leader based on utility scores.
func (vs *ValidatorSet) ElectLeader(metricsCollector *metrics.Collector) {
	vs.mutex.Lock()
	defer vs.mutex.Unlock()

	if len(vs.validators) == 0 {
		vs.logger.Warn("No validators to elect leader from")
		return
	}

	var highestScore float64
	var leaderID peer.ID

	for id := range vs.validators {
		score := metricsCollector.CalculateUtilityScore(id)
		if score > highestScore {
			highestScore = score
			leaderID = id
		}
	}

	if leaderID == "" {
		vs.logger.Warn("No leader elected based on utility scores")
		return
	}

	vs.leaderID = leaderID
	vs.logger.Debug("Elected leader based on utility score", "leaderID", vs.leaderID.String(), "score", highestScore)
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
	if vs.quorumThreshold > 0 {
		return vs.quorumThreshold
	}
	// Default quorum threshold is two-thirds of the total validators
	return (2*len(vs.validators))/3 + 1
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
