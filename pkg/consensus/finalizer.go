package consensus

// BlockFinalizer defines an interface for finalizing blocks.
type BlockFinalizer interface {
	Finalize(blockHash []byte, blockData []byte) error
}
