package consensus

import (
	"encoding/json"
	"errors"
	"math/rand"
	"time"

	"github.com/MariusVanDerWijden/raft-ethereum/engine"
	"github.com/MariusVanDerWijden/raft-ethereum/network"
	"github.com/ethereum/go-ethereum/core/types"
)

type RaftServer struct {
	network   network.API
	execution engine.API

	currentTerm int
	votedFor    int // -1 if no vote yet

	id            int
	numPeers      int
	currentLeader int

	votesForMe     int
	voteInProgress bool

	timer *time.Timer
}

func NewRaft(network network.API, execution engine.API, id int, numPeers int) (*RaftServer, error) {
	raft := RaftServer{
		network:       network,
		execution:     execution,
		currentTerm:   0,
		votedFor:      -1,
		id:            id,
		currentLeader: -1,
		timer:         time.NewTimer(time.Hour),
		numPeers:      numPeers,
	}
	network.RegisterCallback(raft.handleMsg)

	return &raft, nil
}

func (r *RaftServer) Start() error {
	r.resetTimer()
	go r.loop()
	return nil
}

func (r *RaftServer) loop() {
	for {
		if r.currentLeader != r.id {
			select {
			case <-r.timer.C:
				r.forceElection()
				r.resetTimer()
			}
		} else {
			// Send an empty heartbeat
			r.sendHeartbeat(nil)
			// Prepare a block
			block, err := r.execution.GetBlock()
			if err != nil {
				panic(err)
			}
			r.sendHeartbeat(block)
		}
	}
}

func (r *RaftServer) resetTimer() {
	// Reset the timer by 100-150 ms
	rng := rand.Int31n(50)
	r.timer.Reset(time.Duration(100+rng) * time.Millisecond)
}

func (r *RaftServer) forceElection() error {
	// There's a vote already happening, don't force another one
	if r.voteInProgress {
		return nil
	}
	r.voteInProgress = true
	// Assemble vote for self
	vote := Vote{
		Term:         r.currentTerm + 1,
		CandidateID:  r.id,
		HighestBlock: r.execution.LatestBlock(),
	}
	msg, err := vote.MarshalMessage()
	if err != nil {
		return err
	}
	// Broadcast to peers
	if errs := r.network.WriteMsg(msg); len(errs) != 0 {
		return errs[0]
	}
	return nil
}

func (r *RaftServer) handleVote(vote Vote, peer int) {
	r.voteInProgress = true
	var resp VoteResp
	if vote.Term < r.currentTerm || vote.HighestBlock < r.execution.LatestBlock() {
		resp.result = false
	} else {
		resp.result = true
	}
	msg, err := resp.MarshalMessage()
	if err != nil {
		return
	}
	if err := r.network.WriteMsgToPeer(msg, peer); err != nil {
		return
	}
	r.votedFor = vote.CandidateID
}

func (r *RaftServer) tallyVote(voteResp VoteResp, peer int) {
	r.voteInProgress = true
	if voteResp.result {
		r.votesForMe++
	}
	if r.votesForMe >= (r.numPeers / 2) {
		r.currentLeader = r.id
		r.voteInProgress = false
		// Send empty heartbeat to notify of election results
		r.sendHeartbeat(nil)
	}
}

func (r *RaftServer) sendHeartbeat(block *types.Block) error {
	msg := Heartbeat{
		Term:     r.currentTerm,
		LeaderID: r.id,
		Block:    block,
	}
	b, _ := msg.MarshalMessage()
	if errs := r.network.WriteMsg(b); len(errs) != 0 {
		return errs[0]
	}
	return nil
}

func (r *RaftServer) handleMsg(msg []byte, peer int) {
	var packet Message
	if err := json.Unmarshal(msg, &packet); err != nil {
		return
	}
	switch packet.Type {
	case vote_type:
		var vote Vote
		if err := json.Unmarshal(packet.Message, &vote); err != nil {
			return
		}
		r.handleVote(vote, peer)
	case vote_resp_type:
		var voteResp VoteResp
		if err := json.Unmarshal(packet.Message, &voteResp); err != nil {
			return
		}
		r.tallyVote(voteResp, peer)
	case heartbeat_type:
		var heartbeat Heartbeat
		if err := json.Unmarshal(packet.Message, &heartbeat); err != nil {
			return
		}
		if err := r.handleHeartbeat(heartbeat); err != nil {
			panic(err)
		}
	}
}

func (r *RaftServer) handleHeartbeat(heartbeat Heartbeat) error {
	if heartbeat.Term < r.currentTerm {
		// todo handle
		return errors.New("outdated")
	}
	r.currentLeader = heartbeat.LeaderID
	r.voteInProgress = false
	r.resetTimer()
	if heartbeat.Block != nil {
		return r.execution.NewBlock(heartbeat.Block)
	}
	return nil
}
