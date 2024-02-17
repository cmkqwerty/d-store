package p2p

import "net"

// RPC holds any arbitrary data that is being sent over each transport between nodes.
type RPC struct {
	From    net.Addr
	Payload []byte
}
