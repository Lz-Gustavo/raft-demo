package main

import (
	"bytes"
	"encoding/binary"
	"log"
	"strconv"
	"strings"

	"github.com/Lz-Gustavo/beelog/pb"

	"github.com/golang/protobuf/proto"
)

var (
	initValue = []byte(strings.Repeat("!", initValueSize))
)

// MockState ...
type MockState struct {
	state map[string][]byte
}

// NewMockState ...
func NewMockState() *MockState {
	m := &MockState{
		state: make(map[string][]byte, numInitKeys),
	}

	if preInitialize {
		for i := 0; i < numInitKeys; i++ {
			m.state[strconv.Itoa(i)] = initValue
		}
	}
	return m
}

// InstallReceivedState ...
func (m *MockState) InstallReceivedState(newState []byte) (int64, error) {

	reader := bytes.NewReader(newState)
	commandsToApply := make([]pb.Command, 0)
	for {

		var commandLength int32
		err := binary.Read(reader, binary.BigEndian, &commandLength)
		if err != nil {
			log.Println("Error encountered on size reading:", err.Error())
			break
		}

		serializedCmd := make([]byte, commandLength)
		_, err = reader.Read(serializedCmd)
		if err != nil {
			log.Println("Error encountered on buf reading.:", err.Error())
			break
		}

		command := &pb.Command{}
		err = proto.Unmarshal(serializedCmd, command)
		if err != nil {
			log.Println("Error encountered on cmd marsh reading.:", err.Error())
			break
		}
		commandsToApply = append(commandsToApply, *command)
	}

	for _, cmd := range commandsToApply {
		switch cmd.Op {
		case pb.Command_SET:
			m.state[cmd.Key] = []byte(cmd.Value)

		default:
			break
		}
	}

	numOfCommands := int64(len(commandsToApply))
	return numOfCommands, nil
}
