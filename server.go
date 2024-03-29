package main

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"fmt"
	"github.com/cmkqwerty/d-store/p2p"
	"io"
	"log"
	"sync"
	"time"
)

type FileServerOpts struct {
	// ID of the owner of the storage, which will be used to store all files
	// at that location. We can sync all files if needed.
	ID string

	EncKey            []byte
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

	if len(opts.ID) == 0 {
		opts.ID = generateID()
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
	ID   string
	Key  string
	Size int64
}

type MessageGetFile struct {
	ID  string
	Key string
}

type MessageDeleteFile struct {
	ID  string
	Key string
}

type MessageDeleteFileSuccess struct {
	ID  string
	Key string
}

func (fs *FileServer) broadcast(msg *Message) error {
	buf := new(bytes.Buffer)
	if err := gob.NewEncoder(buf).Encode(msg); err != nil {
		return err
	}

	for _, peer := range fs.peers {
		peer.Send([]byte{p2p.IncomingMessage})
		if err := peer.Send(buf.Bytes()); err != nil {
			return err
		}
	}

	return nil
}

func (fs *FileServer) Get(key string) (io.Reader, error) {
	if fs.store.Has(fs.ID, key) {
		fmt.Printf("[%s] serving file (%s) locally, reading from disk...\n", fs.Transport.Addr(), key)
		_, r, err := fs.store.Read(fs.ID, key)

		return r, err
	}

	fmt.Printf("[%s] do not have file (%s) locally, broadcasting request to peers...\n", fs.Transport.Addr(), key)

	msg := Message{
		Payload: MessageGetFile{
			ID:  fs.ID,
			Key: hashKey(key),
		},
	}

	if err := fs.broadcast(&msg); err != nil {
		return nil, err
	}

	time.Sleep(5 * time.Millisecond)

	for _, peer := range fs.peers {
		// First read the filesize, so we can limit the amount of
		// bytes that we read from the connection.
		var fileSize int64
		binary.Read(peer, binary.LittleEndian, &fileSize)

		n, err := fs.store.WriteDecrypt(fs.EncKey, fs.ID, key, io.LimitReader(peer, fileSize))
		if err != nil {
			return nil, err
		}

		fmt.Printf("[%s] received (%d) bytes over the network from: [%s]\n", fs.Transport.Addr(), n, peer.RemoteAddr().String())

		peer.CloseStream()
	}

	_, r, err := fs.store.Read(fs.ID, key)

	return r, err
}

func (fs *FileServer) Store(key string, r io.Reader) error {
	var (
		fileBuffer = new(bytes.Buffer)
		tee        = io.TeeReader(r, fileBuffer)
	)

	size, err := fs.store.Write(fs.ID, key, tee)
	if err != nil {
		return err
	}

	msg := Message{
		Payload: MessageStoreFile{
			ID:   fs.ID,
			Key:  hashKey(key),
			Size: size + 16, // Add 16 bytes for the IV.
		},
	}

	if err := fs.broadcast(&msg); err != nil {
		return err
	}

	time.Sleep(5 * time.Millisecond)

	peers := []io.Writer{}
	for _, peer := range fs.peers {
		peers = append(peers, peer)
	}

	mw := io.MultiWriter(peers...)
	mw.Write([]byte{p2p.IncomingStream})
	n, err := copyEncrypt(fs.EncKey, fileBuffer, mw)
	if err != nil {
		return err
	}

	fmt.Printf("[%s] received and written (%d) bytes to disk.", fs.Transport.Addr(), n)

	return nil
}

func (fs *FileServer) Delete(key string) error {
	// Delete file locally first.
	err := fs.store.Delete(fs.ID, key)
	if err != nil {
		return err
	}

	// Then broadcast delete message to all peers.
	msg := Message{
		Payload: MessageDeleteFile{
			ID:  fs.ID,
			Key: hashKey(key),
		},
	}

	return fs.broadcast(&msg)
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
		log.Println("FileServer stopped due to error or user quit action")
		fs.Transport.Close()
	}()

	for {
		select {
		case rpc := <-fs.Transport.Consume():
			var msg Message
			if err := gob.NewDecoder(bytes.NewReader(rpc.Payload)).Decode(&msg); err != nil {
				log.Printf("Failed to decode payload: %s\n", err)
			}

			if err := fs.handleMessage(rpc.From.String(), &msg); err != nil {
				log.Printf("Failed to handle message: %s\n", err)
			}

		case <-fs.quitch:
			return
		}
	}
}

func (fs *FileServer) handleMessage(from string, msg *Message) error {
	switch v := msg.Payload.(type) {
	case MessageStoreFile:
		return fs.handleMessageStoreFile(from, v)
	case MessageGetFile:
		return fs.handleMessageGetFile(from, v)
	case MessageDeleteFile:
		return fs.handleMessageDeleteFile(from, v)
	case MessageDeleteFileSuccess:
		return fs.handleMessageDeleteFileSuccess(from, v)
	}

	return nil
}

func (fs *FileServer) handleMessageGetFile(from string, msg MessageGetFile) error {
	if !fs.store.Has(msg.ID, msg.Key) {
		return fmt.Errorf("[%s] need to serve file (%s) but it does not exist on disk", fs.Transport.Addr(), msg.Key)
	}

	fmt.Printf("[%s] serving file (%s) over the network.\n", fs.Transport.Addr(), msg.Key)

	fileSize, r, err := fs.store.Read(msg.ID, msg.Key)
	if err != nil {
		return err
	}

	if rc, ok := r.(io.ReadCloser); ok {
		fmt.Println("Closing readCloser")
		defer rc.Close()
	}

	peer, ok := fs.peers[from]
	if !ok {
		return fmt.Errorf("peer %s could not be found.\n", from)
	}

	// First send the incoming stream byte to the peer,
	// and then we can send file size as an int64.
	peer.Send([]byte{p2p.IncomingStream})
	binary.Write(peer, binary.LittleEndian, fileSize)
	n, err := io.Copy(peer, r)
	if err != nil {
		return err
	}

	fmt.Printf("[%s] written [%d] bytes over the network to: %s\n", fs.Transport.Addr(), n, from)

	return nil
}

func (fs *FileServer) handleMessageStoreFile(from string, msg MessageStoreFile) error {
	peer, ok := fs.peers[from]
	if !ok {
		return fmt.Errorf("peer (%s) could not be found.\n", from)
	}

	n, err := fs.store.Write(msg.ID, msg.Key, io.LimitReader(peer, msg.Size))
	if err != nil {
		return err
	}

	fmt.Printf("[%s] written (%d) bytes to disk.\n", fs.Transport.Addr(), n)

	peer.CloseStream()

	return nil
}

func (fs *FileServer) handleMessageDeleteFile(from string, msg MessageDeleteFile) error {
	err := fs.store.Delete(msg.ID, msg.Key)
	if err != nil {
		fmt.Println("Failed to delete file: ", err)
		return err
	}

	peer, ok := fs.peers[from]
	if !ok {
		return fmt.Errorf("peer (%s) could not be found.\n", from)
	}

	confirmMessage := Message{
		Payload: MessageDeleteFileSuccess{
			ID:  msg.ID,
			Key: msg.Key,
		},
	}

	buf := new(bytes.Buffer)
	if err := gob.NewEncoder(buf).Encode(confirmMessage); err != nil {
		return err
	}

	peer.Send([]byte{p2p.IncomingMessage})
	if err := peer.Send(buf.Bytes()); err != nil {
		return err
	}

	return nil
}

func (fs *FileServer) handleMessageDeleteFileSuccess(from string, msg MessageDeleteFileSuccess) error {
	fmt.Printf("[%s] file (%s) has been deleted from peer: %s\n", fs.Transport.Addr(), msg.Key, from)
	return nil
}

func (fs *FileServer) bootstrapNetwork() error {
	for _, addr := range fs.BootstrapNodes {
		if len(addr) == 0 {
			continue
		}

		fmt.Println(fs.Transport.Addr(), "attempting to connect remote: ", addr)
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
	gob.Register(MessageGetFile{})
	gob.Register(MessageDeleteFile{})
	gob.Register(MessageDeleteFileSuccess{})
}
