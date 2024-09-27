package networking

import (
	"context"
	"fmt"
	"log"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
)

// PubSubService manages PubSub operations for the network.
type PubSubService struct {
	PubSub *pubsub.PubSub
	Topic  *pubsub.Topic
	Sub    *pubsub.Subscription
	Ctx    context.Context
	Logger *log.Logger
}

// NewPubSubService creates a new PubSubService for a given topic.
func NewPubSubService(ctx context.Context, ps *pubsub.PubSub, topicName string, logger *log.Logger) (*PubSubService, error) {
	// Join the topic.
	topic, err := ps.Join(topicName)
	if err != nil {
		return nil, fmt.Errorf("failed to join topic: %w", err)
	}

	// Subscribe to the topic.
	sub, err := topic.Subscribe()
	if err != nil {
		return nil, fmt.Errorf("failed to subscribe to topic: %w", err)
	}

	return &PubSubService{
		PubSub: ps,
		Topic:  topic,
		Sub:    sub,
		Ctx:    ctx,
		Logger: logger,
	}, nil
}

// PublishMessage publishes a message to the PubSub topic.
func (ps *PubSubService) PublishMessage(message []byte) error {
	return ps.Topic.Publish(ps.Ctx, message)
}

// ReadMessages reads messages from the PubSub topic.
func (ps *PubSubService) ReadMessages() {
	for {
		msg, err := ps.Sub.Next(ps.Ctx)
		if err != nil {
			ps.Logger.Printf("Failed to read message: %v", err)
			continue
		}
		ps.Logger.Printf("Received message from %s: %s", msg.GetFrom(), string(msg.Data))
	}
}
