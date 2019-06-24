package main

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/Lz-Gustavo/journey"
	"github.com/Lz-Gustavo/journey/pb"
	"github.com/golang/protobuf/proto"
	"github.com/hashicorp/raft"
)

type fsm Store

// Apply applies a Raft log entry to the key-value store.
func (f *fsm) Apply(l *raft.Log) interface{} {

	// TODO: Extend protobuffs for command interpretation too, instead of
	// addhoc message formats
	if f.Logging {
		cmd, _ := serializeCommandInProtobuf(string(l.Data), l.Index)
		defer f.recov.Put(cmd)
	}

	message := string(l.Data)
	message = strings.TrimSuffix(message, "\n")
	req := strings.Split(message, "-")

	switch req[1] {
	case "set":
		return f.applySet(req[2], req[3])
	case "delete":
		return f.applyDelete(req[2])
	case "get":
		return f.applyGet(req[2])
	default:
		panic(fmt.Sprintf("unrecognized command op: %s", req[1]))
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

func (f *fsm) applySet(key, value string) interface{} {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.m[key] = value
	return nil
}

func (f *fsm) applyDelete(key string) interface{} {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.m, key)
	return nil
}

func (f *fsm) applyGet(key string) interface{} {
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

func serializeCommandInJSON(requistion string, index uint64) ([]byte, error) {

	lowerCase := strings.ToLower(requistion)
	lowerCase = strings.TrimSuffix(lowerCase, "\n")
	content := strings.Split(lowerCase, "-")

	var op journey.Operation
	cmt := content[2]

	switch content[1] {
	case "set":
		op = journey.Set
		cmt = strings.Join(content[2:4], "-")
	case "get":
		op = journey.Get
	case "delete":
		op = journey.Delete
	default:
		return nil, fmt.Errorf("Failed to serialize command, operation %q not recognized", content[1])
	}

	cmd := &journey.Command{
		Id:      index,
		Ip:      content[0],
		Op:      op,
		Comment: cmt,
	}

	s, err := json.Marshal(cmd)
	if err != nil {
		return nil, err
	}
	return s, nil
}

func serializeCommandInProtobuf(requistion string, index uint64) ([]byte, error) {

	lowerCase := strings.ToLower(requistion)
	lowerCase = strings.TrimSuffix(lowerCase, "\n")
	content := strings.Split(lowerCase, "-")

	var op pb.Command_Operation
	var value string
	key := content[2]

	switch content[1] {
	case "set":
		op = pb.Command_SET
		value = content[3]
	case "get":
		op = pb.Command_GET
	case "delete":
		op = pb.Command_DELETE
	default:
		return nil, fmt.Errorf("Failed to serialize command, operation %q not recognized", content[1])
	}
	cmd := &pb.Command{
		Id:    index,
		Ip:    content[0],
		Op:    op,
		Key:   key,
		Value: value,
	}

	bytes, err := proto.Marshal(cmd)
	if err != nil {
		return nil, err
	}
	return bytes, nil
}
