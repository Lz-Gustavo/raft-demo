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
	content := strings.Split(message, "-")

	switch content[1] {
	case "set":
		s.recov.Put(l.Index, journey.Set, content[2]+"-"+content[3], content[0], time.Now().Format(time.Stamp))
	case "get":
		s.recov.Put(l.Index, journey.Get, content[2], content[0], time.Now().Format(time.Stamp))
	case "delete":
		s.recov.Put(l.Index, journey.Delete, content[2], content[0], time.Now().Format(time.Stamp))
	}
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
