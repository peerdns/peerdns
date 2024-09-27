// pkg/consensus/errors.go
package consensus

import "errors"

// Consensus-specific errors for better error handling.
var (
	ErrNotLeader        = errors.New("current validator is not the leader")
	ErrInvalidProposal  = errors.New("invalid block proposal")
	ErrQuorumNotReached = errors.New("quorum not reached for finalization")
	ErrMessageNotFound  = errors.New("consensus message not found in pool")
	ErrInvalidSignature = errors.New("invalid signature in consensus message")
	ErrStorageFailure   = errors.New("failed to persist data to storage")
)
