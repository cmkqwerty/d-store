package main

import (
	"bytes"
	"testing"
)

func TestCopyEncryptDecrypt(t *testing.T) {
	payload := "hello world"
	key := newEncryptionKey()
	src := bytes.NewReader([]byte(payload))
	dst := new(bytes.Buffer)
	_, err := copyEncrypt(key, src, dst)
	if err != nil {
		t.Fatal(err)
	}

	out := new(bytes.Buffer)
	nw, err := copyDecrypt(key, dst, out)
	if err != nil {
		t.Fatal(err)
	}

	if nw != len(payload)+16 {
		t.Fatal("decrypted data is not the expected length")
	}

	if out.String() != payload {
		t.Fatal("decrypted string does not match original")
	}
}
