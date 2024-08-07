package network

type Callback func(message []byte, peer int)

type API interface {
	Start() error
	WriteMsg(msg []byte) []error
	WriteMsgToPeer(msg []byte, peer int) error
	RegisterCallback(onMessage Callback)
}
