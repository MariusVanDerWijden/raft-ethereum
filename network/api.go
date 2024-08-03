package network

type API interface {
	WriteMsg(msg []byte) []error
	WriteMsgToPeer(msg []byte, peer int) error
	RegisterCallback(onMessage func(msg []byte, sender int))
}
