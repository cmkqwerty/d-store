package main

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"github.com/cmkqwerty/d-store/p2p"
	"io"
	"log"
	"sync"
)

type FileServerOpts struct {
	StorageRoot       string
	PathTransformFunc PathTransformFunc
	Transport         p2p.Transport
	BootstrapNodes    []string
}

type FileServer struct {
	FileServerOpts

	peerLock sync.RWMutex
	peers    map[string]p2p.Peer

	store  *Store
	quitch chan struct{}
}

func NewFileServer(opts FileServerOpts) *FileServer {
	storeOpts := StoreOpts{
		Root:              opts.StorageRoot,
		PathTransformFunc: opts.PathTransformFunc,
	}

	return &FileServer{
		FileServerOpts: opts,
		store:          NewStore(storeOpts),
		quitch:         make(chan struct{}),
		peers:          make(map[string]p2p.Peer),
	}
}

type Message struct {
	Payload any
}

type MessageStoreFile struct {
	Key string
}

//type DataMessage struct {
//	Key  string
//	Data []byte
//}

func (fs *FileServer) broadcast(msg *Message) error {
	var peers []io.Writer
	for _, peer := range fs.peers {
		peers = append(peers, peer)
	}

	mw := io.MultiWriter(peers...)

	return gob.NewEncoder(mw).Encode(msg)
}

func (fs *FileServer) StoreData(key string, r io.Reader) error {
	//buf := new(bytes.Buffer)
	//tee := io.TeeReader(r, buf)
	//
	//if err := fs.store.Write(key, tee); err != nil {
	//	return err
	//}
	//
	//p := &DataMessage{
	//	Key:  key,
	//	Data: buf.Bytes(),
	//}
	//
	//return fs.broadcast(&Message{
	//	From:    "TODO",
	//	Payload: p,
	//})

	buf := new(bytes.Buffer)

	msg := Message{
		Payload: MessageStoreFile{Key: key},
	}

	if err := gob.NewEncoder(buf).Encode(msg); err != nil {
		return err
	}

	for _, peer := range fs.peers {
		if err := peer.Send(buf.Bytes()); err != nil {
			return err
		}
	}

	//time.Sleep(2 * time.Second)
	//
	//payload := []byte("VERY_LARGE_DATA")
	//
	//for _, peer := range fs.peers {
	//	if err := peer.Send(payload); err != nil {
	//		return err
	//	}
	//}

	return nil
}

func (fs *FileServer) Stop() {
	close(fs.quitch)
}

func (fs *FileServer) OnPeer(p p2p.Peer) error {
	fs.peerLock.Lock()
	defer fs.peerLock.Unlock()

	fs.peers[p.RemoteAddr().String()] = p

	log.Printf("Peer connected: %s", p.RemoteAddr().String())

	return nil
}

func (fs *FileServer) loop() {
	defer func() {
		log.Println("FileServer stopped due to user quit action")
		fs.Transport.Close()
	}()

	for {
		select {
		case rpc := <-fs.Transport.Consume():
			var msg Message
			if err := gob.NewDecoder(bytes.NewReader(rpc.Payload)).Decode(&msg); err != nil {
				log.Printf("Failed to decode payload: %s\n", err)
				return
			}

			fmt.Printf("Received message: %+v\n", msg)

			peer, ok := fs.peers[rpc.From.String()]
			if !ok {
				panic("peer not found")
			}

			b := make([]byte, 1024)
			if _, err := peer.Read(b); err != nil {
				panic(err)
			}

			fmt.Printf("Received message: %s\n", string(b))

			peer.(*p2p.TCPPeer).Wg.Done()

			//if err := fs.handleMessage(&m); err != nil {
			//	log.Printf("Failed to handle message: %s\n", err)
			//}
		case <-fs.quitch:
			return
		}
	}
}

//func (fs *FileServer) handleMessage(msg *Message) error {
//	switch v := msg.Payload.(type) {
//	case *DataMessage:
//		fmt.Printf("Received data message: %s\n", v.Key)
//	}
//
//	return nil
//}

func (fs *FileServer) bootstrapNetwork() error {
	for _, addr := range fs.BootstrapNodes {
		if len(addr) == 0 {
			continue
		}

		fmt.Println("attempting to connect remote: ", addr)
		go func(addr string) {
			if err := fs.Transport.Dial(addr); err != nil {
				log.Printf("Failed to dial %s: %s", addr, err)
			}
		}(addr)
	}

	return nil
}
func (fs *FileServer) Start() error {
	if err := fs.Transport.ListenAndAccept(); err != nil {
		return err
	}

	fs.bootstrapNetwork()

	fs.loop()

	return nil
}

func init() {
	gob.Register(MessageStoreFile{})
}
