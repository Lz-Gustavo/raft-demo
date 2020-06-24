package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
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
func (m *MockState) InstallReceivedState(newState []byte) (uint64, error) {
	rd := bytes.NewReader(newState)
	var f, l uint64

	_, err := fmt.Fscanf(rd, "%d\n%d\n", &f, &l)
	if err != nil {
		return 0, err
	}
	cmds := make([]pb.Command, 0, l-f)

	for {
		var cmdLen int32
		err := binary.Read(rd, binary.BigEndian, &cmdLen)
		if err == io.EOF {
			break
		} else if err != nil {
			return 0, err
		}

		raw := make([]byte, cmdLen)
		_, err = rd.Read(raw)
		if err == io.EOF {
			break
		} else if err != nil {
			return 0, err
		}

		c := &pb.Command{}
		err = proto.Unmarshal(raw, c)
		if err != nil {
			return 0, err
		}
		cmds = append(cmds, *c)
	}

	// apply received commands on mock state
	for _, cmd := range cmds {
		switch cmd.Op {
		case pb.Command_SET:
			m.state[cmd.Key] = []byte(cmd.Value)

		default:
			break
		}
	}
	return uint64(len(cmds)), nil
}
