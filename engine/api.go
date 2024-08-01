package engine

import (
	"github.com/ethereum/go-ethereum/core/types"
)

type API interface {
	NewBlock(block *types.Block) error
	GetBlock() (*types.Block, error)
}
