package main

import (
	"fmt"
	"strconv"
	"strings"
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
func (m *MockState) InstallReceivedState(newState []byte) error {
	// TODO: ...
	fmt.Println("Received State:", newState)
	return nil
}
