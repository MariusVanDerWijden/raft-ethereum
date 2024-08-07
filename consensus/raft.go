package consensus

import (
	"math/rand"
	"time"

	"github.com/MariusVanDerWijden/raft-ethereum/engine"
	"github.com/MariusVanDerWijden/raft-ethereum/network"
	"github.com/ethereum/go-ethereum/log"
)

const NO_LEADER = -1

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
		votedFor:        NO_LEADER,
		id:              id,
		currentLeader:   NO_LEADER,
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
	r.setLeader(NO_LEADER)
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

func (r *RaftServer) setLeader(leader int) {
	if leader == NO_LEADER {
		r.voteInProgress = true
		r.currentLeader = NO_LEADER
	} else {
		r.voteInProgress = false
		r.currentLeader = leader
	}
}
