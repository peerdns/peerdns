// pkg/consensus/message_pool.go
package consensus

import (
	"fmt"
	"log"
	"sync"
)

// ConsensusMessagePool stores and manages in-flight consensus messages.
type ConsensusMessagePool struct {
	messages map[string]*ConsensusMessage // Map of messages indexed by block hash
	logger   *log.Logger                  // Logger for tracking message pool activity
	mutex    sync.RWMutex                 // Mutex for safe access
}

// NewConsensusMessagePool creates a new ConsensusMessagePool.
func NewConsensusMessagePool(logger *log.Logger) *ConsensusMessagePool {
	return &ConsensusMessagePool{
		messages: make(map[string]*ConsensusMessage),
		logger:   logger,
	}
}

// AddMessage adds a new message to the pool.
func (mp *ConsensusMessagePool) AddMessage(msg *ConsensusMessage) {
	mp.mutex.Lock()
	defer mp.mutex.Unlock()
	mp.messages[string(msg.BlockHash)] = msg
	mp.logger.Printf("Added message to pool: %s", msg.BlockHash)
}

// GetMessage retrieves a message by block hash.
func (mp *ConsensusMessagePool) GetMessage(blockHash []byte) (*ConsensusMessage, error) {
	mp.mutex.RLock()
	defer mp.mutex.RUnlock()
	message, exists := mp.messages[string(blockHash)]
	if !exists {
		return nil, fmt.Errorf("message for block hash %x not found", blockHash)
	}
	return message, nil
}

// RemoveMessage removes a message from the pool by block hash.
func (mp *ConsensusMessagePool) RemoveMessage(blockHash []byte) {
	mp.mutex.Lock()
	defer mp.mutex.Unlock()
	delete(mp.messages, string(blockHash))
	mp.logger.Printf("Removed message from pool: %x", blockHash)
}

// HasMessage checks if a message exists in the pool by block hash.
func (mp *ConsensusMessagePool) HasMessage(blockHash []byte) bool {
	mp.mutex.RLock()
	defer mp.mutex.RUnlock()
	_, exists := mp.messages[string(blockHash)]
	return exists
}

// ListMessages returns a list of all messages in the pool.
func (mp *ConsensusMessagePool) ListMessages() []*ConsensusMessage {
	mp.mutex.RLock()
	defer mp.mutex.RUnlock()
	messages := make([]*ConsensusMessage, 0, len(mp.messages))
	for _, msg := range mp.messages {
		messages = append(messages, msg)
	}
	return messages
}
