// pkg/state/state.go
package state

import (
	"fmt"
	"log"
	"sync"

	"github.com/peerdns/peerdns/pkg/storage"
)

// Manager manages the immutable state using MDBX for storage.
type Manager struct {
	db  *storage.Db
	mu  sync.RWMutex
	log *log.Logger
}

// NewStateManager initializes a new StateManager with the given storage manager instance.
func NewStateManager(db *storage.Db, logger *log.Logger) *Manager {
	return &Manager{
		db:  db, // Use the provided storage.Manager
		log: logger,
	}
}

// Close gracefully closes the underlying storage.
func (sm *Manager) Close() error {
	return sm.db.Close()
}

// AddProposal adds a new proposal to the state. Proposals are immutable once added.
func (sm *Manager) AddProposal(proposal *Proposal) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	key := fmt.Sprintf("proposal:%s", proposal.ID)
	exists, err := sm.db.Exists([]byte(key))
	if err != nil {
		return fmt.Errorf("failed to check proposal existence: %w", err)
	}
	if exists {
		return fmt.Errorf("proposal with ID %s already exists", proposal.ID)
	}

	encoded, err := proposal.Encode()
	if err != nil {
		return fmt.Errorf("failed to encode proposal: %w", err)
	}

	if err := sm.db.Set([]byte(key), encoded); err != nil {
		return fmt.Errorf("failed to store proposal: %w", err)
	}

	sm.log.Printf("Added proposal ID: %s, BlockHash: %x", proposal.ID, proposal.BlockHash)
	return nil
}

// GetProposal retrieves a proposal by its ID.
func (sm *Manager) GetProposal(id string) (*Proposal, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	key := fmt.Sprintf("proposal:%s", id)
	data, err := sm.db.Get([]byte(key))
	if err != nil {
		return nil, fmt.Errorf("failed to get proposal: %w", err)
	}

	proposal, err := DecodeProposal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to decode proposal: %w", err)
	}

	return proposal, nil
}

// ListProposals retrieves all proposals.
func (sm *Manager) ListProposals() ([]*Proposal, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	keys, err := sm.db.ListKeysWithPrefix("proposal:")
	if err != nil {
		return nil, fmt.Errorf("failed to list proposal keys: %w", err)
	}

	var proposals []*Proposal
	for _, key := range keys {
		data, err := sm.db.Get([]byte(key))
		if err != nil {
			sm.log.Printf("Failed to get proposal for key %s: %v", key, err)
			continue
		}
		proposal, err := DecodeProposal(data)
		if err != nil {
			sm.log.Printf("Failed to decode proposal for key %s: %v", key, err)
			continue
		}
		proposals = append(proposals, proposal)
	}

	return proposals, nil
}

// AddPeer adds a new peer to the state.
func (sm *Manager) AddPeer(peer *Peer) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	key := fmt.Sprintf("peer:%s", peer.ID)
	exists, err := sm.db.Exists([]byte(key))
	if err != nil {
		return fmt.Errorf("failed to check peer existence: %w", err)
	}
	if exists {
		return fmt.Errorf("peer with ID %s already exists", peer.ID)
	}

	encoded, err := peer.Encode()
	if err != nil {
		return fmt.Errorf("failed to encode peer: %w", err)
	}

	if err := sm.db.Set([]byte(key), encoded); err != nil {
		return fmt.Errorf("failed to store peer: %w", err)
	}

	sm.log.Printf("Added peer ID: %s, Address: %s", peer.ID, peer.Address)
	return nil
}

// RemovePeer marks a peer as removed by setting the LeftAt timestamp.
func (sm *Manager) RemovePeer(id string, leftAt int64) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	key := fmt.Sprintf("peer:%s", id)
	data, err := sm.db.Get([]byte(key))
	if err != nil {
		return fmt.Errorf("failed to get peer: %w", err)
	}

	peer, err := DecodePeer(data)
	if err != nil {
		return fmt.Errorf("failed to decode peer: %w", err)
	}

	if peer.LeftAt != nil {
		return fmt.Errorf("peer %s is already removed", id)
	}

	peer.LeftAt = &leftAt
	encoded, err := peer.Encode()
	if err != nil {
		return fmt.Errorf("failed to encode updated peer: %w", err)
	}

	if err := sm.db.Set([]byte(key), encoded); err != nil {
		return fmt.Errorf("failed to update peer: %w", err)
	}

	sm.log.Printf("Removed peer ID: %s at %d", peer.ID, leftAt)
	return nil
}

// GetPeer retrieves a peer by its ID.
func (sm *Manager) GetPeer(id string) (*Peer, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	key := fmt.Sprintf("peer:%s", id)
	data, err := sm.db.Get([]byte(key))
	if err != nil {
		return nil, fmt.Errorf("failed to get peer: %w", err)
	}

	peer, err := DecodePeer(data)
	if err != nil {
		return nil, fmt.Errorf("failed to decode peer: %w", err)
	}

	return peer, nil
}

// ListPeers retrieves all active peers.
func (sm *Manager) ListPeers() ([]*Peer, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	keys, err := sm.db.ListKeysWithPrefix("peer:")
	if err != nil {
		return nil, fmt.Errorf("failed to list peer keys: %w", err)
	}

	var peers []*Peer
	for _, key := range keys {
		data, err := sm.db.Get([]byte(key))
		if err != nil {
			sm.log.Printf("Failed to get peer for key %s: %v", key, err)
			continue
		}
		peer, err := DecodePeer(data)
		if err != nil {
			sm.log.Printf("Failed to decode peer for key %s: %v", key, err)
			continue
		}
		if peer.LeftAt == nil {
			peers = append(peers, peer)
		}
	}

	return peers, nil
}

// RecordLatency records the latency for a peer.
func (sm *Manager) RecordLatency(record *LatencyRecord) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	key := fmt.Sprintf("latency:%s:%d", record.PeerID, record.Timestamp)
	encoded, err := record.Encode()
	if err != nil {
		return fmt.Errorf("failed to encode latency record: %w", err)
	}

	if err := sm.db.Set([]byte(key), encoded); err != nil {
		return fmt.Errorf("failed to store latency record: %w", err)
	}

	sm.log.Printf("Recorded latency for peer ID: %s, Latency: %d ms", record.PeerID, record.LatencyMs)
	return nil
}

// GetLatencyRecords retrieves all latency records for a peer.
func (sm *Manager) GetLatencyRecords(peerID string) ([]*LatencyRecord, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	prefix := fmt.Sprintf("latency:%s:", peerID)
	keys, err := sm.db.ListKeysWithPrefix(prefix)
	if err != nil {
		return nil, fmt.Errorf("failed to list latency keys: %w", err)
	}

	var records []*LatencyRecord
	for _, key := range keys {
		data, err := sm.db.Get([]byte(key))
		if err != nil {
			sm.log.Printf("Failed to get latency record for key %s: %v", key, err)
			continue
		}
		record, err := DecodeLatencyRecord(data)
		if err != nil {
			sm.log.Printf("Failed to decode latency record for key %s: %v", key, err)
			continue
		}
		records = append(records, record)
	}

	return records, nil
}

// AddFinalization records the finalization of a block.
func (sm *Manager) AddFinalization(finalization *Finalization) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	key := fmt.Sprintf("finalization:%x", finalization.BlockHash)
	exists, err := sm.db.Exists([]byte(key))
	if err != nil {
		return fmt.Errorf("failed to check finalization existence: %w", err)
	}
	if exists {
		return fmt.Errorf("finalization for block hash %x already exists", finalization.BlockHash)
	}

	encoded, err := finalization.Encode()
	if err != nil {
		return fmt.Errorf("failed to encode finalization: %w", err)
	}

	if err := sm.db.Set([]byte(key), encoded); err != nil {
		return fmt.Errorf("failed to store finalization: %w", err)
	}

	sm.log.Printf("Finalized block hash: %x at %d", finalization.BlockHash, finalization.Timestamp)
	return nil
}

// GetFinalization retrieves the finalization record for a block hash.
func (sm *Manager) GetFinalization(blockHash []byte) (*Finalization, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	key := fmt.Sprintf("finalization:%x", blockHash)
	data, err := sm.db.Get([]byte(key))
	if err != nil {
		return nil, fmt.Errorf("failed to get finalization: %w", err)
	}

	finalization, err := DecodeFinalization(data)
	if err != nil {
		return nil, fmt.Errorf("failed to decode finalization: %w", err)
	}

	return finalization, nil
}

// ListFinalizations retrieves all finalization records.
func (sm *Manager) ListFinalizations() ([]*Finalization, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	keys, err := sm.db.ListKeysWithPrefix("finalization:")
	if err != nil {
		return nil, fmt.Errorf("failed to list finalization keys: %w", err)
	}

	var finalizations []*Finalization
	for _, key := range keys {
		data, err := sm.db.Get([]byte(key))
		if err != nil {
			sm.log.Printf("Failed to get finalization for key %s: %v", key, err)
			continue
		}
		finalization, err := DecodeFinalization(data)
		if err != nil {
			sm.log.Printf("Failed to decode finalization for key %s: %v", key, err)
			continue
		}
		finalizations = append(finalizations, finalization)
	}

	return finalizations, nil
}
