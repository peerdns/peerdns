// pkg/consensus/message_pool.go
package consensus

import (
	"fmt"
	"github.com/peerdns/peerdns/pkg/messages"
	"log"
	"sync"
)

// ConsensusMessagePool stores and manages in-flight consensus messages.
type ConsensusMessagePool struct {
	messages map[string]*messages.ConsensusMessage // Map of messages indexed by block hash
	logger   *log.Logger                           // Logger for tracking message pool activity
	mutex    sync.RWMutex                          // Mutex for safe access
}

// NewConsensusMessagePool creates a new ConsensusMessagePool.
func NewConsensusMessagePool(logger *log.Logger) *ConsensusMessagePool {
	return &ConsensusMessagePool{
		messages: make(map[string]*messages.ConsensusMessage),
		logger:   logger,
	}
}

// AddMessage adds a new message to the pool.
func (mp *ConsensusMessagePool) AddMessage(msg *messages.ConsensusMessage) {
	mp.mutex.Lock()
	defer mp.mutex.Unlock()
	mp.messages[string(msg.BlockHash)] = msg
	mp.logger.Printf("Added message to pool: %s", msg.BlockHash)
}

// GetMessage retrieves a message by block hash.
func (mp *ConsensusMessagePool) GetMessage(blockHash []byte) (*messages.ConsensusMessage, error) {
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
func (mp *ConsensusMessagePool) ListMessages() []*messages.ConsensusMessage {
	mp.mutex.RLock()
	defer mp.mutex.RUnlock()
	msgs := make([]*messages.ConsensusMessage, 0, len(mp.messages))
	for _, msg := range mp.messages {
		msgs = append(msgs, msg)
	}
	return msgs
}
