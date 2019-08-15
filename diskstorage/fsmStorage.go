package main

import (
	"fmt"
	"io"
	"strconv"
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
		defer f.LogFile.Write(serializedCmd)
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

	pos, err := strconv.Atoi(key)
	if err != nil {
		panic(fmt.Sprintf("Could not interpret specified key value %q", key))
	}
	stride := int64(f.valueSize * pos)

	_, err = f.Local.WriteAt(f.storeValue, stride)
	if err != nil {
		panic(err.Error())
	}
	return ""
}

func (f *fsm) applyDelete(key string) string {

	pos, err := strconv.Atoi(key)
	if err != nil {
		panic(fmt.Sprintf("Could not interpret specified key value %q", key))
	}
	stride := int64(f.valueSize * pos)

	blankContent := []byte(strings.Repeat(" ", f.valueSize))
	_, err = f.Local.WriteAt(blankContent, stride)
	if err != nil {
		panic(err.Error())
	}
	return ""
}

func (f *fsm) applyGet(key string) string {

	pos, err := strconv.Atoi(key)
	if err != nil {
		panic(fmt.Sprintf("Could not interpret specified key value %q", key))
	}
	stride := int64(f.valueSize * pos)

	content := make([]byte, f.valueSize, f.valueSize)
	_, err = f.Local.ReadAt(content, stride)
	if err != nil {
		panic(err.Error())
	}
	return string(content)
}

type fsmSnapshot struct {
	store []byte
}

func (f *fsmSnapshot) Persist(sink raft.SnapshotSink) error {
	return nil
}

func (f *fsmSnapshot) Release() {}
