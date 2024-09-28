package networking

import (
	"context"
	"github.com/peerdns/peerdns/pkg/messages"
	"log"
	"sync"

	"github.com/libp2p/go-libp2p/core/peer"
)

// MockSubscription is a mock implementation of the Subscription interface.
type MockSubscription struct {
	mu       sync.Mutex
	messages [][]byte
	closed   bool
}

// NewMockSubscription initializes and returns a new MockSubscription.
func NewMockSubscription() *MockSubscription {
	return &MockSubscription{
		messages: make([][]byte, 0),
	}
}

// EnqueueMessage allows tests to enqueue messages that will be returned by Next().
func (ms *MockSubscription) EnqueueMessage(message []byte) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	ms.messages = append(ms.messages, message)
}

// Next returns the next message in the subscription.
// If no messages are available, it returns nil without blocking.
func (ms *MockSubscription) Next(ctx context.Context) (*Message, error) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	if ms.closed {
		return nil, context.Canceled
	}

	if len(ms.messages) == 0 {
		return nil, nil
	}

	msg := ms.messages[0]
	ms.messages = ms.messages[1:]

	return &Message{Data: msg}, nil
}

// Cancel marks the subscription as closed.
func (ms *MockSubscription) Cancel() {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	ms.closed = true
}

// MockP2PNetwork is a mock implementation of the P2PNetworkInterface.
type MockP2PNetwork struct {
	mu            sync.Mutex
	broadcasted   [][]byte
	subscriptions map[string]*MockSubscription
	hostID        peer.ID
	broadcastCh   chan struct{}
	approvalCh    chan struct{} // New channel for approvals
	logger        *log.Logger
}

// NewMockP2PNetwork initializes a new MockP2PNetwork.
func NewMockP2PNetwork(hostID peer.ID, logger *log.Logger) *MockP2PNetwork {
	return &MockP2PNetwork{
		broadcasted:   make([][]byte, 0),
		subscriptions: make(map[string]*MockSubscription),
		hostID:        hostID,
		broadcastCh:   make(chan struct{}, 100),
		approvalCh:    make(chan struct{}, 100),
		logger:        logger,
	}
}

// Ch returns the broadcast channel.
func (m *MockP2PNetwork) Ch() <-chan struct{} {
	return m.broadcastCh
}

// ApprovalCh returns the approval channel.
func (m *MockP2PNetwork) ApprovalCh() <-chan struct{} {
	return m.approvalCh
}

// BroadcastMessage records the broadcasted message and enqueues it to all subscriptions.
func (m *MockP2PNetwork) BroadcastMessage(message []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.broadcasted = append(m.broadcasted, message)

	// Enqueue the message into all subscriptions
	for _, sub := range m.subscriptions {
		sub.EnqueueMessage(message)
	}

	m.logger.Printf("Broadcasted message: %x", message)

	// Detect ApprovalMessage and signal via approvalCh
	msg, err := messages.DeserializeConsensusMessage(message)
	if err == nil && msg.Type == messages.ApprovalMessage {
		select {
		case m.approvalCh <- struct{}{}:
		default:
		}
	}

	// Notify that a message was broadcasted
	select {
	case m.broadcastCh <- struct{}{}:
	default:
		// If the channel is full, avoid blocking
	}

	return nil
}

// PubSubSubscribe returns a mock subscription for the given topic.
func (m *MockP2PNetwork) PubSubSubscribe(topic string) (Subscription, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	mockSub, exists := m.subscriptions[topic]
	if !exists {
		mockSub = NewMockSubscription()
		m.subscriptions[topic] = mockSub
	}
	return mockSub, nil
}

// GetBroadcastedMessages returns all messages that have been broadcasted.
func (m *MockP2PNetwork) GetBroadcastedMessages() [][]byte {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.broadcasted
}

// EnqueueMessage enqueues a message to a specific topic's subscription.
func (m *MockP2PNetwork) EnqueueMessage(topic string, message []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()

	mockSub, exists := m.subscriptions[topic]
	if !exists {
		mockSub = NewMockSubscription()
		m.subscriptions[topic] = mockSub
	}
	mockSub.EnqueueMessage(message)
}

// HostID returns the mock host's peer ID.
func (m *MockP2PNetwork) HostID() peer.ID {
	return m.hostID
}
