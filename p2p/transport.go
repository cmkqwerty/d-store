package p2p

// Peer is an interface that represents the remote node.
type Peer interface {
	Close() error
}

// Transport is anything that handles communication between nodes in the network.
// This can be of the form TCP, UDP, WebSockets, etc.
type Transport interface {
	ListenAndAccept() error
	Consume() <-chan RPC
	Close() error
}
