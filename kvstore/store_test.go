package main

import (
	"context"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/Lz-Gustavo/beelog/pb"

	"github.com/golang/protobuf/proto"
)

func TestCreate(t *testing.T) {
	s := New(context.TODO(), false)
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
	s := New(context.TODO(), true)
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

	cmd := &pb.Command{
		Op:    pb.Command_SET,
		Key:   "foo",
		Value: "bar",
	}
	bytes, _ := proto.Marshal(cmd)
	if err := s.Propose(bytes, nil, ""); err != nil {
		t.Fatalf("failed to set key: %s", err.Error())
	}

	// Wait for committed log entry to be applied.
	time.Sleep(500 * time.Millisecond)
	value := s.testGet("foo")
	if value != "bar" {
		t.Fatalf("key has wrong value: %s", value)
	}

	cmd = &pb.Command{
		Op:  pb.Command_DELETE,
		Key: "foo",
	}
	bytes, _ = proto.Marshal(cmd)
	if err := s.Propose(bytes, nil, ""); err != nil {
		t.Fatalf("failed to delete key: %s", err.Error())
	}

	// Wait for committed log entry to be applied.
	time.Sleep(500 * time.Millisecond)
	value = s.testGet("foo")
	if value != "" {
		t.Fatalf("key has wrong value: %s", value)
	}
}
