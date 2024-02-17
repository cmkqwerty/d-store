package main

import (
	"bytes"
	"testing"
)

func TestPathTransformFunc(t *testing.T) {
	key := "my-special-picture"
	expectedOriginalKey := "ea842a6587df2f0961463b74cde7959393acd483"
	expectedPathName := "ea842/a6587/df2f0/96146/3b74c/de795/9393a/cd483"
	actual := CASPathTransformFunc(key)

	if actual.PathName != expectedPathName {
		t.Fatalf("expected %s, got %s", expectedPathName, actual.PathName)
	}
	if actual.Original != expectedOriginalKey {
		t.Fatalf("expected %s, got %s", expectedOriginalKey, actual.Original)
	}
}

func TestStore(t *testing.T) {
	opts := StoreOpts{
		PathTransformFunc: CASPathTransformFunc,
	}
	s := NewStore(opts)

	data := bytes.NewReader([]byte("hello, world"))
	if err := s.writeStream("my-special-picture", data); err != nil {
		t.Fatal(err)
	}
}
