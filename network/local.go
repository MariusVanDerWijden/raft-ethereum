package network

import (
	"io"
)

type LocalNetwork struct {
	inputs    []io.Reader
	outputs   []io.Writer
	callbacks []Callback
}

// NewLocalNetwork creates a fully connected network
// with io.pipes.
func NewLocalNetwork(numNodes int) []LocalNetwork {
	networks := make([]LocalNetwork, numNodes)

	for i := 0; i < numNodes; i++ {
		for k := 0; k < numNodes; k++ {
			if i == k {
				// Don't need a pipe between the same peer
				continue
			}
			in, out := io.Pipe()
			networks[i].inputs = append(networks[i].inputs, in)
			networks[k].outputs = append(networks[k].outputs, out)
		}
	}
	return networks
}

func (s *LocalNetwork) Start() error {
	// Start up the listening processes
	for id, conn := range s.inputs {
		go Listen(conn, s.callbacks, id)
	}
	return nil
}

func (s *LocalNetwork) WriteMsg(msg []byte) []error {
	return BroadcastMsg(s.outputs, msg)
}

func (s *LocalNetwork) WriteMsgToPeer(msg []byte, peer int) error {
	return WriteMsg(s.outputs[peer], msg)
}

func (s *LocalNetwork) RegisterCallback(onMessage Callback) {
	s.callbacks = append(s.callbacks, onMessage)
}
