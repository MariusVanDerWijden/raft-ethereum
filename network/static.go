package network

import "net"

var _ = API(&StaticNetwork{})

type StaticNetwork struct {
	// TODO properly lock the peers array and callbacks array
	peers     []net.Conn
	callbacks []func(message []byte, peer int)
}

func NewStaticNetwork(port string) (*StaticNetwork, error) {
	network := StaticNetwork{}
	listener, err := net.Listen("tcp4", port)
	if err != nil {
		return nil, err
	}
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				network.peers = append(network.peers, conn)
			}
		}
	}()
	go network.loop()

	return &network, nil
}

func (s *StaticNetwork) loop() error {
	for {
		for id, conn := range s.peers {
			buffer := make([]byte, 128)
			if _, err := conn.Read(buffer); err != nil {
				// handle error
			} else {
				for _, callback := range s.callbacks {
					callback(buffer, id)
				}
			}
		}
	}
}

func (s *StaticNetwork) AddPeer(address string) error {
	addr, err := net.ResolveTCPAddr("tcp", address)
	if err != nil {
		return err
	}
	conn, err := net.DialTCP("tcp", nil, addr)
	if err != nil {
		return err
	}
	s.peers = append(s.peers, conn)
	return nil
}

func (s *StaticNetwork) WriteMsg(msg []byte) []error {
	var errs []error
	for _, conn := range s.peers {
		if _, err := conn.Write(msg); err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}

func (s *StaticNetwork) WriteMsgToPeer(msg []byte, peer int) error {
	_, err := s.peers[peer].Write(msg)
	return err
}

func (s *StaticNetwork) RegisterCallback(onMessage func(msg []byte, peer int)) {
	s.callbacks = append(s.callbacks, onMessage)
}
