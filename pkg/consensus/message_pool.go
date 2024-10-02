package consensus

import (
	"fmt"
	"sync"

	"github.com/peerdns/peerdns/pkg/logger"
	"github.com/peerdns/peerdns/pkg/packets"
)

// MessagePool stores and manages in-flight consensus packets.
type MessagePool struct {
	messages map[string]*packets.ConsensusPacket // Map of messages indexed by block hash
	logger   logger.Logger                       // Logger for tracking message pool activity
	mutex    sync.RWMutex                        // Mutex for safe access
}

// NewMessagePool creates a new MessagePool.
func NewMessagePool(logger logger.Logger) *MessagePool {
	return &MessagePool{
		messages: make(map[string]*packets.ConsensusPacket),
		logger:   logger,
	}
}

// AddMessage adds a new consensus packet to the pool.
func (mp *MessagePool) AddMessage(msg *packets.ConsensusPacket) {
	mp.mutex.Lock()
	defer mp.mutex.Unlock()
	mp.messages[string(msg.BlockHash)] = msg
	mp.logger.Debug("Added message to pool", "blockHash", fmt.Sprintf("%x", msg.BlockHash))
}

// GetMessage retrieves a consensus packet by block hash.
func (mp *MessagePool) GetMessage(blockHash []byte) (*packets.ConsensusPacket, error) {
	mp.mutex.RLock()
	defer mp.mutex.RUnlock()
	message, exists := mp.messages[string(blockHash)]
	if !exists {
		return nil, fmt.Errorf("message for block hash %x not found", blockHash)
	}
	return message, nil
}

// RemoveMessage removes a consensus packet from the pool by block hash.
func (mp *MessagePool) RemoveMessage(blockHash []byte) {
	mp.mutex.Lock()
	defer mp.mutex.Unlock()
	delete(mp.messages, string(blockHash))
	mp.logger.Debug("Removed message from pool", "blockHash", fmt.Sprintf("%x", blockHash))
}

// HasMessage checks if a consensus packet exists in the pool by block hash.
func (mp *MessagePool) HasMessage(blockHash []byte) bool {
	mp.mutex.RLock()
	defer mp.mutex.RUnlock()
	_, exists := mp.messages[string(blockHash)]
	return exists
}

// ListMessages returns a list of all consensus packets in the pool.
func (mp *MessagePool) ListMessages() []*packets.ConsensusPacket {
	mp.mutex.RLock()
	defer mp.mutex.RUnlock()
	msgs := make([]*packets.ConsensusPacket, 0, len(mp.messages))
	for _, msg := range mp.messages {
		msgs = append(msgs, msg)
	}
	return msgs
}
