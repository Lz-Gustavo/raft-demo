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

	firstIndex = 0
	lastIndex  = 10000
)

var (
	// Used to initialize a state transfer protocol to the application log after a specified
	// number of seconds.
	sleepDuration int
	recovAddr     string
)

func init() {
	flag.IntVar(&sleepDuration, "sleep", 60, "set the countdown for a state request, defaults to 1min")
	flag.StringVar(&recovAddr, "recov", ":14000", "set an address to request state, defaults to localhost:14000")
}

func main() {
	flag.Parse()
	validIP := recovAddr != ""
	if !validIP {
		log.Fatalln("must set a valid IP address to request state, run with: ./recovery -recov 'ipAddress'")
	}

	recovReplica := NewMockState()

	// Wait for the application to log a sequence of commands
	time.Sleep(time.Duration(sleepDuration) * time.Second)

	fmt.Printf("Asking for interval: [%d, %d]\n", firstIndex, lastIndex)
	recvState, dur := AskForStateTransfer(firstIndex, lastIndex)
	numCommands, stateInstallTime := StartStateInstallation(recovReplica, recvState)

	fmt.Println("Transfer time (ns):", dur)
	fmt.Println("Install time (ns):", stateInstallTime)
	fmt.Println("State size (bytes):", len(recvState))
	fmt.Println("Num of commands:", numCommands)
}

// AskForStateTransfer ...
func AskForStateTransfer(p, n uint64) ([]byte, uint64) {
	f := strconv.FormatUint(p, 10)
	l := strconv.FormatUint(n, 10)

	start := time.Now()
	recvState, err := sendStateRequest(f, l)
	if err != nil {
		log.Fatalf("failed to receive a new state from node '%s', error: %s", recovAddr, err.Error())
	}
	finish := uint64(time.Since(start) / time.Nanosecond)
	return recvState, finish
}

// StartStateInstallation ...
func StartStateInstallation(replica *MockState, recvState []byte) (numCmds, duration uint64) {
	// In order to identify the received interval without a stdout during timing
	var p, n uint64
	start := time.Now()

	cmds, err := replica.InstallReceivedState(recvState, &p, &n)
	if err != nil {
		log.Fatalf("failed to install the received state: %s", err.Error())
	}
	finish := uint64(time.Since(start) / time.Nanosecond)

	fmt.Printf("Received interval: [%d, %d]\n", p, n)
	return cmds, finish
}

func sendStateRequest(first, last string) ([]byte, error) {
	stateConn, err := net.Dial("tcp", recovAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to node at '%s', error: %s", recovAddr, err.Error())
	}

	reqMsg := stateConn.LocalAddr().String() + "-" + first + "-" + last + "\n"
	_, err = fmt.Fprint(stateConn, reqMsg)
	if err != nil {
		return nil, fmt.Errorf("failed sending state request to node at '%s', error: %s", recovAddr, err.Error())
	}

	var recv []byte
	recv, err = ioutil.ReadAll(stateConn)
	if err != nil {
		log.Fatalln("could not read state response:", err.Error())
	}

	if err = stateConn.Close(); err != nil {
		return nil, err
	}
	return recv, nil
}
