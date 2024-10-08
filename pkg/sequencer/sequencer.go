// pkg/sequencer/sequencer.go

package sequencer

import (
	"context"
	"errors"
	"github.com/peerdns/peerdns/pkg/logger"
	"go.uber.org/zap"
	"time"
)

// Sequencer manages the process of pulling transactions from the TxPool and forging blocks.
type Sequencer struct {
	stateMgr    *StateManager
	ctx         context.Context
	cancel      context.CancelFunc
	forgeTicker *time.Ticker
	logger      logger.Logger
	producer    *BlockProducer
}

// NewSequencer initializes a new Sequencer instance.
func NewSequencer(ctx context.Context, producer *BlockProducer, logger logger.Logger, metrics *SequencerMetrics) *Sequencer {
	ctx, cancel := context.WithCancel(ctx)
	stateMgr := NewStateManager(logger, metrics)

	return &Sequencer{
		stateMgr:    stateMgr,
		logger:      logger,
		ctx:         ctx,
		cancel:      cancel,
		producer:    producer,
		forgeTicker: nil, // Will be initialized in Start
	}
}

// Start begins the sequencer's operations.
func (s *Sequencer) Start() error {
	if s.stateMgr == nil {
		return errors.New("state manager not initialized")
	}

	s.stateMgr.SetState(SequencerStateType, Initializing)
	s.stateMgr.SetState(ProducerStateType, Initializing)

	// Initialize components (e.g., set up tickers for forging)
	s.forgeTicker = time.NewTicker(500 * time.Millisecond)

	s.stateMgr.SetState(SequencerStateType, Started)
	s.logger.Info("Sequencer started successfully")

	// Start the block producer loop
	go s.produce()

	return nil
}

// produce periodically triggers the BlockProducer to produce blocks.
func (s *Sequencer) produce() {
	for {
		select {
		case <-s.ctx.Done():
			s.logger.Info("Sequencer shutting down")
			s.stateMgr.SetState(SequencerStateType, Stopping)
			if s.forgeTicker != nil {
				s.forgeTicker.Stop()
			}
			s.stateMgr.SetState(SequencerStateType, Stopped)
			return
		case <-s.forgeTicker.C:
			s.logger.Debug("Attempting to produce a new block")
			s.stateMgr.SetState(ProducerStateType, Starting)
			if err := s.producer.ForgeBlock(); err != nil {
				s.logger.Error("Failed to forge block", zap.Error(err))
			}
		}
	}
}

// Stop gracefully stops the sequencer.
func (s *Sequencer) Stop() error {
	s.cancel()
	s.logger.Info("Sequencer stopped successfully")
	return nil
}
