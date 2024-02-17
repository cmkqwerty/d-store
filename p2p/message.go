package p2p

import "net"

// Message holds any arbitrary data that is being sent over each transport between nodes.
type Message struct {
	From    net.Addr
	Payload []byte
}
