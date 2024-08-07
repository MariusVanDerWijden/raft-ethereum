package network

import (
	"bytes"
	"fmt"
	"io"
	"sync/atomic"
	"testing"
	"time"
)

func read(conn io.Reader) ([]byte, error) {
	buffer := make([]byte, 5*1024*1024)
	n, err := conn.Read(buffer)
	if err != nil {
		return buffer, err
	}
	return buffer[:n], nil
}

func TestLocalNetworkBroadcast(t *testing.T) {
	nw := NewLocalNetwork(2)
	go func() {
		nw[1].WriteMsg([]byte{0xff})
	}()
	res, err := read(nw[0].inputs[0])
	if err != nil {
		panic(err)
	}
	if !bytes.Equal(res, []byte{0xff}) {
		panic(fmt.Sprintf("%x", res))
	}
}

func TestLocalNetworkUnicast(t *testing.T) {
	nw := NewLocalNetwork(2)
	go func() {
		nw[1].WriteMsgToPeer([]byte{0xff}, 0)
	}()
	res, err := read(nw[0].inputs[0])
	if err != nil {
		panic(err)
	}
	if !bytes.Equal(res, []byte{0xff}) {
		panic(fmt.Sprintf("%x", res))
	}
}

func TestConnectivity(t *testing.T) {
	nw := NewLocalNetwork(5)
	for _, n := range nw {
		if len(n.inputs) != 4 {
			panic(len(n.inputs))
		}
	}
}

func TestBroadcasts(t *testing.T) {
	nw := NewLocalNetwork(3)
	go func() {
		nw[1].WriteMsg([]byte{0xff})
	}()
	res, err := read(nw[0].inputs[0])
	if err != nil {
		panic(err)
	}
	if !bytes.Equal(res, []byte{0xff}) {
		panic(fmt.Sprintf("%x", res))
	}
	res, err = read(nw[2].inputs[1])
	if err != nil {
		panic(err)
	}
	if !bytes.Equal(res, []byte{0xff}) {
		panic(fmt.Sprintf("%x", res))
	}
}

func TestRegisterCallback(t *testing.T) {
	nw := NewLocalNetwork(3)
	var called atomic.Int32
	callback := func(msg []byte, peer int) {
		if !bytes.Equal(msg, []byte{0xff}) {
			panic(fmt.Sprintf("%x", msg))
		}
		called.Add(1)
	}
	nw[1].RegisterCallback(callback)
	nw[2].RegisterCallback(callback)
	nw[1].Start()
	nw[2].Start()
	go func() {
		nw[0].WriteMsg([]byte{0xff})
	}()
	time.Sleep(100 * time.Millisecond)
	if called.Load() != 2 {
		panic(fmt.Sprintf("%d", called.Load()))
	}
}
