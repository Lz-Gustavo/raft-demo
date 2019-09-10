package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"strconv"
	"time"
)

const (
	preInitialize = false
	numInitKeys   = 1000000
	initValueSize = 128

	// Used to initialize a state transfer protocol to the application log after a specified
	// number of seconds.
	requestStateAfterSec = 30
)

var recovAddr string

func init() {
	flag.StringVar(&recovAddr, "recov", "", "Set an address to request state")
}

func main() {
	flag.Parse()

	//validIP := net.ParseIP(recovAddr) != nil
	validIP := recovAddr != ""
	if !validIP {
		log.Fatalln("Must set a valid IP address to request state, run with: ./recovery -recov 'ipAddress'")
	}

	recovReplica := NewMockState()

	// Wait for the application to log a sequence of commands
	time.Sleep(time.Duration(requestStateAfterSec) * time.Second)

	receivedState, stateTransferTime := AskForStateTransfer(0)

	numCommands, stateInstallTime := StartStateInstallation(recovReplica, receivedState)

	fmt.Println("Transfer Time (ns):", stateTransferTime)
	fmt.Println("Install Time (ns):", stateInstallTime)
	fmt.Println("State Size (bytes):", len(receivedState))
	fmt.Println("Num of Commands:", numCommands)
}

// AskForStateTransfer ...
func AskForStateTransfer(firstIndex int) ([]byte, int64) {

	stateTransferStart := time.Now()
	receivedState, err := sendStateRequest(firstIndex)
	if err != nil {
		log.Fatalf("Failed to receive a new state from node %s: %s", recovAddr, err.Error())
	}
	stateTransferFinish := int64(time.Since(stateTransferStart) / time.Nanosecond)
	return receivedState, stateTransferFinish
}

// StartStateInstallation ...
func StartStateInstallation(replica *MockState, receivedState []byte) (numCmds, duration int64) {

	stateInstallStart := time.Now()
	cmds, err := replica.InstallReceivedState(receivedState)
	if err != nil {
		log.Fatalf("Failed to install the received state: %s", err.Error())
	}
	stateInstallFinish := int64(time.Since(stateInstallStart) / time.Nanosecond)
	return cmds, stateInstallFinish
}

func sendStateRequest(index int) ([]byte, error) {
	stateConn, err := net.Dial("tcp", recovAddr)
	if err != nil {
		return nil, fmt.Errorf("Failed to connect to node at %s: %s", recovAddr, err.Error())
	}

	requestMessage := stateConn.LocalAddr().String() + "-" + strconv.Itoa(index) + "\n"
	_, err = fmt.Fprint(stateConn, requestMessage)
	if err != nil {
		return nil, fmt.Errorf("Failed sending state request to node at %s: %s", recovAddr, err.Error())
	}

	var receivedData []byte

	receivedData, err = ioutil.ReadAll(stateConn)
	if err != nil {
		log.Fatalln("Could not read state response:", err.Error())
	}

	if err = stateConn.Close(); err != nil {
		return nil, err
	}

	return receivedData, nil
}
