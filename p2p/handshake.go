package p2p

// HandshakeFunc is a function that is called when a new connection is established.
type HandshakeFunc func(Peer) error

func NOPHandshakeFunc(Peer) error { return nil }
