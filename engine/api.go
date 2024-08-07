package engine

import (
	"github.com/ethereum/go-ethereum/beacon/engine"
)

type Block = engine.ExecutableData

type API interface {
	NewBlock(block *Block) error
	GetBlock() (*Block, error)
	LatestBlock() int
}
