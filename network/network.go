package network

type API interface {
	SendMessage(msg any)
	RegisterCallback(onMessage func(msg any))
}
