package main

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/Lz-Gustavo/journey/pb"
	"github.com/golang/protobuf/proto"
	"github.com/hashicorp/raft"
)

type fsm Store

// Apply applies a Raft log entry to the key-value store.
func (f *fsm) Apply(l *raft.Log) interface{} {

	command := &pb.Command{}
	err := proto.Unmarshal(l.Data, command)
	if err != nil {
		return err
	}

	if f.Logging {
		command.Id = l.Index
		serializedCmd, _ := proto.Marshal(command)
		defer f.recov.Put(serializedCmd)
	}

	switch command.Op {
	case pb.Command_SET:
		return strings.Join([]string{command.Ip, f.applySet(command.Key, command.Value)}, "-")
	case pb.Command_DELETE:
		return strings.Join([]string{command.Ip, f.applyDelete(command.Key)}, "-")
	case pb.Command_GET:
		return strings.Join([]string{command.Ip, f.applyGet(command.Key)}, "-")
	default:
		panic(fmt.Sprintf("unrecognized command op: %v", command))
	}
}

// Snapshot returns a snapshot of the key-value store.
func (f *fsm) Snapshot() (raft.FSMSnapshot, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Clone the map.
	o := make(map[string]string)
	for k, v := range f.m {
		o[k] = v
	}
	return &fsmSnapshot{store: o}, nil
}

// Restore stores the key-value store to a previous state.
func (f *fsm) Restore(rc io.ReadCloser) error {
	o := make(map[string]string)
	if err := json.NewDecoder(rc).Decode(&o); err != nil {
		return err
	}

	// Set the state from the snapshot, no lock required according to
	// Hashicorp docs.
	f.m = o
	return nil
}

func (f *fsm) applySet(key, value string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.m[key] = value
	return ""
}

func (f *fsm) applyDelete(key string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.m, key)
	return ""
}

func (f *fsm) applyGet(key string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.m[key]
}

type fsmSnapshot struct {
	store map[string]string
}

func (f *fsmSnapshot) Persist(sink raft.SnapshotSink) error {
	err := func() error {
		// Encode data.
		b, err := json.Marshal(f.store)
		if err != nil {
			return err
		}

		// Write data to sink.
		if _, err := sink.Write(b); err != nil {
			return err
		}

		// Close the sink.
		return sink.Close()
	}()

	if err != nil {
		sink.Cancel()
	}

	return err
}

func (f *fsmSnapshot) Release() {}
