package main

import (
	"bytes"
	"github.com/cmkqwerty/d-store/p2p"
	"log"
	"time"
)

func makeServer(listenAddr string, nodes ...string) *FileServer {
	tcpTransportOpts := p2p.TCPTransportOpts{
		ListenAddr:    listenAddr,
		HandshakeFunc: p2p.NOPHandshakeFunc,
		Decoder:       p2p.DefaultDecoder{},
	}
	tcpTransport := p2p.NewTCPTransport(tcpTransportOpts)

	fileServerOpts := FileServerOpts{
		EncKey:            newEncryptionKey(),
		StorageRoot:       listenAddr + "_network",
		PathTransformFunc: CASPathTransformFunc,
		Transport:         tcpTransport,
		BootstrapNodes:    nodes,
	}

	fs := NewFileServer(fileServerOpts)

	tcpTransport.OnPeer = fs.OnPeer

	return fs
}

func main() {
	s1 := makeServer(":3000", "")
	s2 := makeServer(":4000", ":3000")

	go func() {
		log.Fatal(s1.Start())
	}()

	time.Sleep(2 * time.Second)

	go func() {
		log.Fatal(s2.Start())
	}()

	time.Sleep(2 * time.Second)

	data := bytes.NewReader([]byte("My secret big data file!"))
	s2.Store("picture.jpg", data)

	//r, err := s2.Get("picture.jpg")
	//if err != nil {
	//	log.Fatal(err)
	//}
	//
	//b, err := io.ReadAll(r)
	//if err != nil {
	//	log.Fatal(err)
	//}

	//fmt.Println(string(b))
}
