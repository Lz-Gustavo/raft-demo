package main

import (
	"io/ioutil"
	"os"
	"testing"
	"time"
)

func TestCreate(t *testing.T) {
	s := New(false)
	tmpDir, _ := ioutil.TempDir("", "store_test")
	defer os.RemoveAll(tmpDir)

	s.RaftBind = "127.0.0.1:0"
	s.RaftDir = tmpDir
	if s == nil {
		t.Fatalf("failed to create store")
	}

	if err := s.StartRaft(true, "node0", s.RaftBind); err != nil {
		t.Fatalf("failed to open store: %s", err)
	}
}

func TestOperations(t *testing.T) {
	s := New(true)
	tmpDir, _ := ioutil.TempDir("", "store_test")
	defer os.RemoveAll(tmpDir)

	s.RaftBind = "127.0.0.1:0"
	s.RaftDir = tmpDir
	if s == nil {
		t.Fatalf("failed to create store")
	}

	if err := s.StartRaft(true, "node0", s.RaftBind); err != nil {
		t.Fatalf("failed to open store: %s", err)
	}

	// Simple way to ensure there is a leader.
	time.Sleep(3 * time.Second)

	if err := s.Propose("set-foo-bar"); err != nil {
		t.Fatalf("failed to set key: %s", err.Error())
	}

	// Wait for committed log entry to be applied.
	time.Sleep(500 * time.Millisecond)
	value := s.Get("foo")
	if value != "bar" {
		t.Fatalf("key has wrong value: %s", value)
	}

	if err := s.Propose("delete-foo-bar"); err != nil {
		t.Fatalf("failed to delete key: %s", err.Error())
	}

	// Wait for committed log entry to be applied.
	time.Sleep(500 * time.Millisecond)
	value = s.Get("foo")
	if value != "" {
		t.Fatalf("key has wrong value: %s", value)
	}
}
