package main

import (
	"github.com/cmkqwerty/d-store/p2p"
	"log"
	"time"
)

func main() {
	tcpTransportOpts := p2p.TCPTransportOpts{
		ListenAddr:    ":3000",
		HandshakeFunc: p2p.NOPHandshakeFunc,
		Decoder:       p2p.DefaultDecoder{},
		// TODO: onPeer()
	}
	tcpTransport := p2p.NewTCPTransport(tcpTransportOpts)

	fileServerOpts := FileServerOpts{
		StorageRoot:       "3000_network",
		PathTransformFunc: CASPathTransformFunc,
		Transport:         tcpTransport,
	}

	fs := NewFileServer(fileServerOpts)

	go func() {
		time.Sleep(time.Second * 5)
		fs.Stop()
	}()

	if err := fs.Start(); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}
