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

// Must implement the Raft FSM interface, even if the chatRoom application
// won't do any procedure after successfully applys by consensus protocol
type fsm Logger

// Apply proposes a new value to the consensus cluster
func (s *fsm) Apply(l *raft.Log) interface{} {

	command := &pb.Command{}
	err := proto.Unmarshal(l.Data, command)
	if err != nil {
		return err
	}
	command.Id = l.Index
	serializedCmd, _ := proto.Marshal(command)
	_, err = s.LogFile.Write(serializedCmd)
	return err
}

// Restore stores the key-value store to a previous state.
func (s *fsm) Restore(rc io.ReadCloser) error {
	return nil
}

// Snapshot returns a snapshot of the key-value store.
func (s *fsm) Snapshot() (raft.FSMSnapshot, error) {
	return nil, nil
}

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
