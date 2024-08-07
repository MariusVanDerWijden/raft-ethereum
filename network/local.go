package network

import (
	"io"

	"github.com/ethereum/go-ethereum/log"
)

type LocalNetwork struct {
	inputs    []io.Reader
	outputs   []io.Writer
	callbacks []func(message []byte, peer int)
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
	// Start up the listening process
	for id, conn := range s.inputs {
		go func(conn io.Reader, id int) {
			for {
				buffer := make([]byte, 5*1024*1024)
				n, err := conn.Read(buffer)
				if err == nil {
					for _, callback := range s.callbacks {
						callback(buffer[:n], id)
					}
				}
			}
		}(conn, id)
	}
	return nil
}

func (s *LocalNetwork) WriteMsg(msg []byte) []error {
	var errs []error
	for _, conn := range s.outputs {
		if _, err := conn.Write(msg); err != nil {
			log.Warn("Error broadcasting", "error", err)
			errs = append(errs, err)
		}
	}
	return errs
}

func (s *LocalNetwork) WriteMsgToPeer(msg []byte, peer int) error {
	_, err := s.outputs[peer].Write(msg)
	return err
}

func (s *LocalNetwork) RegisterCallback(onMessage func(msg []byte, peer int)) {
	s.callbacks = append(s.callbacks, onMessage)
}
