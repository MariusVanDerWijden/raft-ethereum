package main

import (
	"math/big"
	"os"
	"time"

	"github.com/MariusVanDerWijden/raft-ethereum/consensus"
	"github.com/MariusVanDerWijden/raft-ethereum/engine"
	"github.com/MariusVanDerWijden/raft-ethereum/network"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/eth/ethconfig"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/node"
)

var genAlloc = types.GenesisAlloc{common.Address{}: {Balance: big.NewInt(1000000000000000000)}}

func main() {
	// Enable logging
	log.SetDefault(log.NewLogger(log.NewTerminalHandlerWithLevel(os.Stdout, log.LevelInfo, true)))
	// Setup local network for testing
	localNets := network.NewLocalNetwork(3)
	// Setup three execution layer clients
	a := StartServer(&localNets[0], 0, 2)
	b := StartServer(&localNets[1], 1, 2)
	c := StartServer(&localNets[2], 2, 2)
	// Start them up
	a.Start()
	b.Start()
	c.Start()
	// Let them run
	for {
		time.Sleep(10 * time.Second)
	}
}

func StartServer(network network.API, id, numPeers int) *consensus.RaftServer {
	eng := engine.NewLocal(genAlloc)
	node, err := consensus.NewRaft(network, eng, id, 2)
	if err != nil {
		panic(err)
	}
	return node
}

func WithLogging() func(nodeConf *node.Config, ethConf *ethconfig.Config) {
	return func(nodeConf *node.Config, ethConf *ethconfig.Config) {
		nodeConf.Logger = log.NewLogger(log.JSONHandler(os.Stdout))
	}
}
