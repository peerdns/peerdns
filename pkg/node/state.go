package node

// NodeState represents the various states a node can be in.
type NodeState int

const (
	NodeStateStopped NodeState = iota
	NodeStateStarting
	NodeStateRunning
	NodeStateStopping
	NodeStateFailed
)

// ConsensusState represents the various states the ConsensusModule can be in.
type ConsensusState int

const (
	ConsensusStateStopped ConsensusState = iota
	ConsensusStateStarting
	ConsensusStateRunning
	ConsensusStateStopping
	ConsensusStateFailed
)
