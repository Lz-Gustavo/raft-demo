package main

import (
	"io"
	"journey"
	"strings"
	"time"

	"github.com/hashicorp/raft"
)

// Must implement the Raft FSM interface, even if the chatRoom application
// won't do any procedure after successfully applys by consensus protocol
type fsm Logger

// Apply proposes a new value to the consensus cluster
func (s *fsm) Apply(l *raft.Log) interface{} {

	message := string(l.Data)
	message = strings.TrimSuffix(message, "\n")

	// TODO: must modify recov.Put() to log following journey's semantics. Passing
	// specific operation type and operands
	s.recov.Put(0, journey.Set, message, time.Now().Format(time.Stamp))
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
