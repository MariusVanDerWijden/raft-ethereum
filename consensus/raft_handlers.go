package consensus

import (
	"encoding/json"

	"github.com/MariusVanDerWijden/raft-ethereum/engine"
	"github.com/ethereum/go-ethereum/log"
)

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

func (r *RaftServer) handleVote(vote Vote, peer int) {
	r.setLeader(NO_LEADER)
	var resp VoteResp
	if vote.Term < r.currentTerm || vote.HighestBlock < r.execution.LatestBlock() {
		resp.Result = false
	} else {
		resp.Result = true
	}
	r.WriteMsgToPeer(&resp, peer)
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
		r.setLeader(r.id)
		// Send empty heartbeat to notify of election results
		if err := r.sendHeartbeat(nil); err != nil {
			panic(err)
		}
		// Trigger the state transition
		r.stateTransition <- struct{}{}
	}
}

func (r *RaftServer) handleHeartbeat(heartbeat Heartbeat, peer int) error {
	if heartbeat.Term < r.currentTerm {
		return r.WriteMsgToPeer(&HeartbeatResp{
			Term:   r.currentTerm,
			Result: false,
		}, peer)
	}
	r.setLeader(heartbeat.LeaderID)
	r.resetTimer()
	if len(heartbeat.Block) != 0 {
		block := new(engine.Block)
		if err := block.UnmarshalJSON(heartbeat.Block); err != nil {
			return err
		}
		return r.execution.NewBlock(block)
	}
	log.Debug("Received good heartbeat", "id", r.id, "peer", peer)
	return r.WriteMsgToPeer(&HeartbeatResp{
		Term:   r.currentTerm,
		Result: true,
	}, peer)
}
