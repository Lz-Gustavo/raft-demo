package main

import (
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
	return &fsmSnapshot{[]byte("")}, nil
}

// Restore stores the key-value store to a previous state.
func (f *fsm) Restore(rc io.ReadCloser) error {
	return nil
}

// NOTE: There s no need for mutex acquisition since every new command is garantee to be
// executed in a sequential manner, preserving the replicas coordination.
func (f *fsm) applySet(key, value string) string {

	// TODO: write content to Local file

	f.writeCount++
	if f.writeCount == f.batchSync {
		f.Local.Sync()
		f.writeCount = 0
	}
	return ""
}

func (f *fsm) applyDelete(key string) string {

	// TODO: write blank content to local file

	return ""
}

func (f *fsm) applyGet(key string) string {

	// TODO: read content from local file

	return ""
}

type fsmSnapshot struct {
	store []byte
}

func (f *fsmSnapshot) Persist(sink raft.SnapshotSink) error {
	return nil
}

func (f *fsmSnapshot) Release() {}
