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

// InstallRecovState ...
func (m *MockState) InstallRecovState(newState []byte, f, l *uint64) (uint64, error) {
	rd := bytes.NewReader(newState)
	var ln int
	_, err := fmt.Fscanf(rd, "%d\n%d\n%d\n", f, l, &ln)
	if err != nil {
		return 0, err
	}

	cmds := make([]pb.Command, 0, ln)
	for i := 0; i < ln; i++ {
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

	var eol string
	_, err = fmt.Fscanf(rd, "\n%s\n", &eol)
	if err != nil {
		return 0, err
	}
	if eol != "EOL" {
		return 0, fmt.Errorf("expected EOL flag, got '%s'", eol)
	}

	m.applyCommandLog(cmds)
	return uint64(len(cmds)), nil
}

// InstallRecovStateForMultipleLogs ...
func (m *MockState) InstallRecovStateForMultipleLogs(newState []byte, f, l *uint64) (uint64, error) {
	rd := bytes.NewReader(newState)
	var nLogs int

	// read num of retrieved logs, only on multiple logs config
	_, err := fmt.Fscanf(rd, "%d\n", &nLogs)
	if err != nil {
		return 0, err
	}
	cmds := make([]pb.Command, 0, 256*nLogs)

	for i := 0; i < nLogs; i++ {
		// read the retrieved log interval and num of commands
		var ln int
		_, err := fmt.Fscanf(rd, "%d\n%d\n%d\n", f, l, &ln)
		if err != nil {
			return 0, err
		}

		for j := 0; j < ln; j++ {
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

		var eol string
		_, err = fmt.Fscanf(rd, "\n%s\n", &eol)
		if err != nil {
			return 0, err
		}
		if eol != "EOL" {
			return 0, fmt.Errorf("expected EOL flag, got '%s'", eol)
		}
	}

	// apply commands from ALL retrieved logs (already ordered)
	m.applyCommandLog(cmds)
	return uint64(len(cmds)), nil
}

func (m *MockState) applyCommandLog(log []pb.Command) {
	// apply received commands on mock state
	for _, cmd := range log {
		switch cmd.Op {
		case pb.Command_SET:
			m.state[cmd.Key] = []byte(cmd.Value)

		default:
			break
		}
	}
}
