package main

import (
	"io"

	"github.com/hashicorp/raft"
)

// Must implement the Raft FSM interface, even if the chatRoom application
// won't do any procedure after successfully applys by consensus protocol
type fsm Server

// Apply proposes a new value to the consensus cluster
func (s *fsm) Apply(l *raft.Log) interface{} {
	return nil
}

// Restore stores the key-value store to a previous state.
func (s *fsm) Restore(rc io.ReadCloser) error {
	return nil
}

// Snapshot returns a snapshot of the key-value store.
func (s *fsm) Snapshot() (raft.FSMSnapshot, error) {
	return nil, nil
}
