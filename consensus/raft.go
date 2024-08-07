package consensus

import (
	"encoding/json"
	"math/rand"
	"time"

	"github.com/MariusVanDerWijden/raft-ethereum/engine"
	"github.com/MariusVanDerWijden/raft-ethereum/network"
	"github.com/ethereum/go-ethereum/log"
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

	currentBlock *engine.Block

	// The state transition is triggered whenever a leader turns into a follower
	// or a follower/candidate turns into a leader
	stateTransition chan (struct{})

	timer *time.Timer
}

func NewRaft(network network.API, execution engine.API, id int, numPeers int) (*RaftServer, error) {
	raft := &RaftServer{
		network:         network,
		execution:       execution,
		currentTerm:     0,
		votedFor:        -1,
		id:              id,
		currentLeader:   -1,
		timer:           time.NewTimer(time.Hour),
		numPeers:        numPeers,
		stateTransition: make(chan struct{}),
	}
	network.RegisterCallback(raft.handleMsg)
	network.Start()

	return raft, nil
}

func (r *RaftServer) Start() error {
	log.Info("Starting server", "id", r.id)
	r.resetTimer()
	go r.loop()
	return nil
}

func (r *RaftServer) loop() {
	for {
		if r.currentLeader == r.id {
			r.lead()
		} else {
			r.follow()
		}
	}
}

func (r *RaftServer) follow() {
	select {
	case <-r.timer.C:
		if err := r.forceElection(); err != nil {
			panic(err)
		}
		r.resetTimer()
	case <-r.stateTransition:
		return
	}
}

func (r *RaftServer) lead() {
	// Leader
	stopCh := make(chan struct{})
	go func() {
		timer := time.NewTicker(100 * time.Millisecond)
		timer.Reset(100 * time.Millisecond)
		for {
			select {
			case <-timer.C:
				r.sendHeartbeat(nil)
				timer.Reset(100 * time.Millisecond)
			case <-stopCh:
				// Something happened, reset
				return
			}
		}
	}()
	go func() {
		timer := time.NewTicker(100 * time.Millisecond)
		for {
			select {
			case <-timer.C:
				// Prepare a block
				block, err := r.execution.GetBlock()
				if err != nil {
					panic(err)
				}
				if err := r.execution.NewBlock(block); err != nil {
					panic(err)
				}
				r.sendHeartbeat(block)
			case <-stopCh:
				// Something happened, reset
				return
			}
		}
	}()
	// If a state transition was triggered, shut down all heartbeats
	<-r.stateTransition
	close(stopCh)
}

func (r *RaftServer) resetTimer() {
	// Reset the timer by 2000 - 2500 ms
	rng := rand.Int31n(500)
	r.timer.Reset(time.Duration(2000+rng) * time.Millisecond)
}

func (r *RaftServer) forceElection() error {
	if r.currentLeader == r.id {
		// looks like we got elected in the meantime
		log.Warn("Got elected during force election")
		return nil
	}
	log.Info("Forcing an Election", "id", r.id)
	r.voteInProgress = true
	r.currentLeader = -1
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
	r.currentLeader = -1
	var resp VoteResp
	if vote.Term < r.currentTerm || vote.HighestBlock < r.execution.LatestBlock() {
		resp.Result = false
	} else {
		resp.Result = true
	}
	msg, err := resp.MarshalMessage()
	if err != nil {
		return
	}
	if err := r.network.WriteMsgToPeer(msg, peer); err != nil {
		return
	}
	r.votedFor = vote.CandidateID
	r.resetTimer()
	r.stateTransition <- struct{}{}
}

func (r *RaftServer) tallyVote(voteResp VoteResp, peer int) {
	log.Info("Received vote from peer", "peer", peer, "result", voteResp.Result)
	if voteResp.Result {
		r.votesForMe++
	}
	if r.votesForMe > (r.numPeers / 2) {
		log.Info("I am the leader now", "id", r.id)
		r.currentLeader = r.id
		r.voteInProgress = false
		// Trigger the state transition
		r.stateTransition <- struct{}{}
		// Send empty heartbeat to notify of election results
		if err := r.sendHeartbeat(nil); err != nil {
			panic(err)
		}
	}
}

func (r *RaftServer) sendHeartbeat(block *engine.Block) error {
	var enc []byte
	if block != nil {
		var err error
		enc, err = block.MarshalJSON()
		if err != nil {
			return err
		}
		// Cache block for eventual resend
		r.currentBlock = block
	}

	msg := Heartbeat{
		Term:     r.currentTerm,
		LeaderID: r.id,
		Block:    enc,
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
		log.Warn("Unmarshalling failed", "err", err)
		return
	}
	log.Debug("Received message", "id", r.id, "type", packet.Type, "len", len(packet.Message))
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
		if err := r.handleHeartbeat(heartbeat, peer); err != nil {
			panic(err)
		}
	case heartbeat_resp_type:
		var resp HeartbeatResp
		if err := json.Unmarshal(packet.Message, &resp); err != nil {
			return
		}
		if !resp.Result && r.currentLeader == r.id {
			// Try to resend
			r.sendHeartbeat(r.currentBlock)
		}
	}
}

func (r *RaftServer) handleHeartbeat(heartbeat Heartbeat, peer int) error {
	if heartbeat.Term < r.currentTerm {
		resp := HeartbeatResp{
			Term:   r.currentTerm,
			Result: false,
		}
		b, _ := resp.MarshalMessage()
		return r.network.WriteMsgToPeer(b, peer)
	}
	r.currentLeader = heartbeat.LeaderID
	r.voteInProgress = false
	r.resetTimer()
	if len(heartbeat.Block) != 0 {
		block := new(engine.Block)
		if err := block.UnmarshalJSON(heartbeat.Block); err != nil {
			return err
		}
		return r.execution.NewBlock(block)
	}
	log.Debug("Received good heartbeat", "id", r.id, "peer", peer)
	resp := HeartbeatResp{
		Term:   r.currentTerm,
		Result: true,
	}
	b, _ := resp.MarshalMessage()
	return r.network.WriteMsgToPeer(b, peer)
}
