package network

import (
	"crypto/ecdsa"

	"github.com/ethereum/go-ethereum/crypto"
)

type Authenticated struct {
	inner API
	key   *ecdsa.PrivateKey
	peers []*ecdsa.PublicKey
}

// NewAuthenticated wraps a network api with ecdsa based authentication
func NewAuthenticated(key *ecdsa.PrivateKey, peers []*ecdsa.PublicKey, inner API) *Authenticated {
	return &Authenticated{
		inner: inner,
		key:   key,
		peers: peers,
	}
}

func (a *Authenticated) Start() error {
	return a.inner.Start()
}

func (a *Authenticated) RegisterCallback(callback Callback) {
	newCallback := func(message []byte, peer int) {
		if len(message) < 65 {
			return
		}
		hash := crypto.Keccak256(message[65:])
		if crypto.VerifySignature(crypto.FromECDSAPub(a.peers[peer]), hash, message[0:65]) {
			callback(message, peer)
		}
	}
	a.inner.RegisterCallback(newCallback)
}

func (a *Authenticated) WriteMsg(msg []byte) []error {
	hash := crypto.Keccak256(msg)
	sig, err := crypto.Sign(hash, a.key)
	if err != nil {
		return []error{err}
	}
	return a.inner.WriteMsg(append(sig, msg...))
}

func (a *Authenticated) WriteMsgToPeer(msg []byte, peer int) error {
	hash := crypto.Keccak256(msg)
	sig, err := crypto.Sign(hash, a.key)
	if err != nil {
		return err
	}
	return a.inner.WriteMsgToPeer(append(sig, msg...), peer)
}
