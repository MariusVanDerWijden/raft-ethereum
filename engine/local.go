package engine

import (
	"crypto/rand"
	"errors"
	"time"

	"github.com/ethereum/go-ethereum/beacon/engine"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/eth"
	"github.com/ethereum/go-ethereum/eth/catalyst"
	"github.com/ethereum/go-ethereum/eth/downloader"
	"github.com/ethereum/go-ethereum/eth/ethconfig"
	"github.com/ethereum/go-ethereum/eth/filters"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rpc"
)

var _ = API(&LocalEngine{})

type LocalEngine struct {
	node      *node.Node
	backend   *eth.Ethereum
	engineAPI *catalyst.ConsensusAPI

	feeRecipient common.Address
	currentState engine.ForkchoiceStateV1

	blockBuildingTimeout time.Duration
}

func NewLocal(alloc types.GenesisAlloc, options ...func(nodeConf *node.Config, ethConf *ethconfig.Config)) *LocalEngine {
	// Create the default configurations for the outer node shell and the Ethereum
	// service to mutate with the options afterwards
	nodeConf := node.DefaultConfig
	nodeConf.DataDir = ""
	nodeConf.P2P = p2p.Config{NoDiscovery: true}

	ethConf := ethconfig.Defaults
	ethConf.Genesis = &core.Genesis{
		Config:   params.AllDevChainProtocolChanges,
		GasLimit: ethconfig.Defaults.Miner.GasCeil,
		Alloc:    alloc,
	}
	ethConf.SyncMode = downloader.FullSync
	ethConf.TxPool.NoLocals = true

	for _, option := range options {
		option(&nodeConf, &ethConf)
	}
	// Assemble the Ethereum stack to run the chain with
	stack, err := node.New(&nodeConf)
	if err != nil {
		panic(err) // this should never happen
	}
	sim, err := newWithNode(stack, &ethConf, 0)
	if err != nil {
		panic(err) // this should never happen
	}
	return sim
}

// newWithNode sets up a simulated backend on an existing node. The provided node
// must not be started and will be started by this method.
func newWithNode(stack *node.Node, conf *eth.Config, blockPeriod uint64) (*LocalEngine, error) {
	backend, err := eth.New(stack, conf)
	if err != nil {
		return nil, err
	}
	// Register the filter system
	filterSystem := filters.NewFilterSystem(backend.APIBackend, filters.Config{})
	stack.RegisterAPIs([]rpc.API{{
		Namespace: "eth",
		Service:   filters.NewFilterAPI(filterSystem),
	}})
	// Start the node
	if err := stack.Start(); err != nil {
		return nil, err
	}
	// Set up the simulated beacon
	beacon, err := catalyst.NewSimulatedBeacon(blockPeriod, backend)
	if err != nil {
		return nil, err
	}
	// Reorg our chain back to genesis
	if err := beacon.Fork(backend.BlockChain().GetCanonicalHash(0)); err != nil {
		return nil, err
	}
	return &LocalEngine{
		node:      stack,
		backend:   backend,
		engineAPI: catalyst.NewConsensusAPI(backend),
	}, nil
}

func (local *LocalEngine) NewBlock(block *types.Block) error {
	payload := engine.BlockToExecutableData(block, common.Big0, []*types.BlobTxSidecar{})
	// Insert the block
	if _, err := local.engineAPI.NewPayloadV4(*payload.ExecutionPayload, []common.Hash{}, &common.Hash{}); err != nil {
		return err
	}
	local.currentState = engine.ForkchoiceStateV1{
		HeadBlockHash:      block.Hash(),
		SafeBlockHash:      block.Hash(),
		FinalizedBlockHash: block.Hash(),
	}
	// Mark it as canonical
	if _, err := local.engineAPI.ForkchoiceUpdatedV3(local.currentState, nil); err != nil {
		return err
	}
	return nil
}

func (local *LocalEngine) GetBlock() (*types.Block, error) {
	var random [32]byte
	rand.Read(random[:])
	// Trigger a new block to be built
	resp, err := local.engineAPI.ForkchoiceUpdatedV3(local.currentState, &engine.PayloadAttributes{
		Timestamp:             uint64(time.Now().Unix()),
		SuggestedFeeRecipient: local.feeRecipient,
		Withdrawals:           []*types.Withdrawal{},
		Random:                random,
		BeaconRoot:            &common.Hash{},
	})
	if err != nil {
		return nil, err
	}
	if resp == engine.STATUS_SYNCING || resp == engine.STATUS_INVALID {
		return nil, errors.New("invalid or syncing")
	}
	time.Sleep(local.blockBuildingTimeout)
	// Retrieve the payload
	envelope, err := local.engineAPI.GetPayloadV4(*resp.PayloadID)
	if err != nil {
		return nil, err
	}
	// TODO this currently prevents blob transactions from working
	return engine.ExecutableDataToBlock(*envelope.ExecutionPayload, []common.Hash{}, &common.Hash{})
}

func (local *LocalEngine) LatestBlock() int {
	return int(local.backend.BlockChain().CurrentBlock().Number.Int64())
}
