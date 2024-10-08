package sequencer

import (
	"github.com/peerdns/peerdns/pkg/chain"
	"github.com/peerdns/peerdns/pkg/logger"
	"github.com/peerdns/peerdns/pkg/txpool"
	"github.com/peerdns/peerdns/pkg/types"
	"go.uber.org/zap"
)

// BlockProducer is responsible for pulling transactions from the TxPool and forging new blocks.
type BlockProducer struct {
	txPool     *txpool.TxPool
	blockchain *chain.Blockchain
	logger     logger.Logger
	stateMgr   *StateManager
}

// NewBlockProducer initializes a new BlockProducer instance.
func NewBlockProducer(txPool *txpool.TxPool, blockchain *chain.Blockchain, logger logger.Logger, stateMgr *StateManager) *BlockProducer {
	return &BlockProducer{
		txPool:     txPool,
		blockchain: blockchain,
		logger:     logger,
		stateMgr:   stateMgr,
	}
}

// ForgeBlock pulls transactions from the TxPool and creates a new block.
func (bp *BlockProducer) ForgeBlock() error {
	bp.stateMgr.SetState(ProducerStateType, Started)

	// Pull transactions from TxPool
	txs := bp.txPool.GetTransactions(100)
	if len(txs) == 0 {
		bp.logger.Debug("No transactions available to forge a block")
		bp.stateMgr.SetState(ProducerStateType, Failed)
		return nil
	}

	// Get the latest block from the blockchain
	latestBlock, err := bp.blockchain.GetLatestBlock()
	if err != nil {
		bp.stateMgr.SetState(ProducerStateType, Failed)
		return err
	}

	// Assemble transactions into a new block
	newBlock, err := types.NewBlock(
		latestBlock.Index+1,
		latestBlock.Hash,
		txs,
		latestBlock.ValidatorID, // Assuming the latest validator creates the new block
		latestBlock.Difficulty,  // Current difficulty
	)
	if err != nil {
		bp.stateMgr.SetState(ProducerStateType, Failed)
		return err
	}

	// TODO: Implement block signing here

	// Add the new block to the blockchain
	if err := bp.blockchain.AddBlock(newBlock); err != nil {
		bp.stateMgr.SetState(ProducerStateType, Failed)
		return err
	}

	bp.logger.Info("Forged a new block",
		zap.Uint64("block_index", newBlock.Index),
		zap.String("block_id", newBlock.Hash.String()),
		zap.Int("tx_count", len(newBlock.Transactions)),
	)

	// Remove included transactions from the TxPool
	for _, tx := range txs {
		bp.txPool.RemoveTransaction(tx.ID)
	}

	bp.stateMgr.SetState(ProducerStateType, Produced)
	return nil
}
