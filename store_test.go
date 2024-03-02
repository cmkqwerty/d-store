package main

import (
	"bytes"
	"fmt"
	"io"
	"testing"
)

func TestPathTransformFunc(t *testing.T) {
	key := "my-special-picture"
	expectedFilename := "ea842a6587df2f0961463b74cde7959393acd483"
	expectedPathName := "ea842/a6587/df2f0/96146/3b74c/de795/9393a/cd483"
	actual := CASPathTransformFunc(key)

	if actual.PathName != expectedPathName {
		t.Fatalf("expected %s, got %s", expectedPathName, actual.PathName)
	}
	if actual.Filename != expectedFilename {
		t.Fatalf("expected %s, got %s", expectedFilename, actual.Filename)
	}
}

func TestStoreDeleteKey(t *testing.T) {
	opts := StoreOpts{
		PathTransformFunc: CASPathTransformFunc,
	}
	s := NewStore(opts)
	key := "my-special-picture"
	data := []byte("hello, world")

	if err := s.writeStream("my-special-picture", bytes.NewReader(data)); err != nil {
		t.Fatal(err)
	}

	if err := s.Delete(key); err != nil {
		t.Fatal(err)
	}
}

func TestStore(t *testing.T) {
	s := newStore()
	defer teardown(t, s)

	for i := 0; i < 50; i++ {
		key := fmt.Sprintf("secret_%d", i)
		data := []byte("hello, world")

		if err := s.writeStream(key, bytes.NewReader(data)); err != nil {
			t.Fatal(err)
		}

		if ok := s.Has(key); !ok {
			t.Fatalf("expected %s to exist", key)
		}

		r, err := s.Read(key)
		if err != nil {
			t.Fatal(err)
		}

		b, err := io.ReadAll(r)
		if err != nil {
			t.Fatal(err)
		}

		if string(b) != string(data) {
			t.Fatalf("expected %s, got %s", data, b)
		}

		if err := s.Delete(key); err != nil {
			t.Fatal(err)
		}

		if ok := s.Has(key); ok {
			t.Fatalf("expected %s to NOT exist", key)
		}
	}
}

func newStore() *Store {
	opts := StoreOpts{
		Root:              "",
		PathTransformFunc: CASPathTransformFunc,
	}

	return NewStore(opts)
}

func teardown(t *testing.T, s *Store) {
	if err := s.Clear(); err != nil {
		t.Fatal(err)
	}
}
