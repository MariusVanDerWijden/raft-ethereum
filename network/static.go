package network

import (
	"net"
	"time"
)

var _ = API(&StaticNetwork{})

const DefaultReadDeadline = 100 * time.Millisecond

type StaticNetwork struct {
	// TODO properly lock the peers array and callbacks array
	port      string
	peers     []net.Conn
	callbacks []Callback
}

func NewStaticNetwork(port string) (*StaticNetwork, error) {
	return &StaticNetwork{port: port}, nil
}

func (s *StaticNetwork) Start() error {
	listener, err := net.Listen("tcp4", s.port)
	if err != nil {
		return err
	}
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				s.peers = append(s.peers, conn)
			}
		}
	}()
	go s.loop()
	return nil
}

func (s *StaticNetwork) loop() error {
	for id, conn := range s.peers {
		conn.SetReadDeadline(time.Now().Add(DefaultReadDeadline))
		go Listen(conn, s.callbacks, id)
	}
	return nil
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
	return WriteMsg(s.peers[peer], msg)
}

func (s *StaticNetwork) RegisterCallback(onMessage Callback) {
	s.callbacks = append(s.callbacks, onMessage)
}
