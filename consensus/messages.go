package consensus

import (
	"encoding/json"
)

const (
	heartbeat_type = iota
	heartbeat_resp_type
	vote_type
	vote_resp_type
)

type Message struct {
	Type    byte
	Message []byte
}

func AsMessage(type_selector byte, message any) ([]byte, error) {
	inner, err := json.Marshal(message)
	if err != nil {
		return nil, err
	}
	msg := Message{
		Type:    type_selector,
		Message: inner,
	}
	return json.Marshal(msg)
}

type Vote struct {
	Term         int
	CandidateID  int
	HighestBlock int
}

func (v *Vote) MarshalMessage() ([]byte, error) {
	return AsMessage(vote_type, v)
}

type VoteResp struct{ Result bool }

func (v *VoteResp) MarshalMessage() ([]byte, error) {
	return AsMessage(vote_resp_type, v)
}

type Heartbeat struct {
	Term     int
	LeaderID int
	Block    []byte
}

func (h *Heartbeat) MarshalMessage() ([]byte, error) {
	return AsMessage(heartbeat_type, h)
}

type HeartbeatResp struct {
	Term   int
	Result bool
}

func (h *HeartbeatResp) MarshalMessage() ([]byte, error) {
	return AsMessage(heartbeat_resp_type, h)
}
