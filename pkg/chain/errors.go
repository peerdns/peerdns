package chain

import "github.com/pkg/errors"

// Errors
var (
	ErrInvalidBlock         = errors.New("invalid block")
	ErrGenesisAlreadyExists = errors.New("genesis block already exists")
	ErrNoGenesisBlock       = errors.New("no genesis block found")
)
