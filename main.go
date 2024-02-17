package main

import (
	"fmt"
	"github.com/cmkqwerty/d-store/p2p"
	"log"
)

func OnPeer(peer p2p.Peer) error {
	fmt.Printf("Peer connected: %+v\n", peer)
	return nil
}

func main() {
	tcpOpts := p2p.TCPTransportOpts{
		ListenAddr:    ":3000",
		HandshakeFunc: p2p.NOPHandshakeFunc,
		Decoder:       p2p.DefaultDecoder{},
		OnPeer:        OnPeer,
	}

	tr := p2p.NewTCPTransport(tcpOpts)

	go func() {
		for {
			msg := <-tr.Consume()
			fmt.Printf("Received message: %+v\n", msg)
		}
	}()

	if err := tr.ListenAndAccept(); err != nil {
		log.Fatalf("Error listening and accepting: %s", err)
	}

	select {}
}
