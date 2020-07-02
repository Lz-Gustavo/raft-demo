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
)

var (
	// Used to initialize a state transfer protocol to the application log after a specified
	// number of seconds.
	sleepDuration         int
	recovAddr             string
	firstIndex, lastIndex string
)

func init() {
	flag.IntVar(&sleepDuration, "sleep", 60, "set the countdown for a state request, defaults to 1min")
	flag.StringVar(&recovAddr, "recov", ":14000", "set an address to request state, defaults to localhost:14000")
	flag.StringVar(&firstIndex, "p", "", "set the first index of requested state")
	flag.StringVar(&lastIndex, "n", "", "set the last index of requested state")
}

func main() {
	flag.Parse()
	validIP := recovAddr != ""
	if !validIP {
		log.Fatalln("must set a valid IP address to request state, run with: ./recovery -recov 'ipAddress'")
	}

	if err := validInterval(firstIndex, lastIndex); err != nil {
		log.Fatalln(err.Error(), "must set a valid interval, run with: ./recovery -p 'num' -n 'num'")
	}

	recovReplica := NewMockState()

	// Wait for the application to log a sequence of commands
	time.Sleep(time.Duration(sleepDuration) * time.Second)

	fmt.Printf("Asking for interval: [%s, %s]\n", firstIndex, lastIndex)
	recvState, dur := AskForStateTransfer(firstIndex, lastIndex)
	numCommands, stateInstallTime := StartStateInstallation(recovReplica, recvState)

	fmt.Println("Transfer time (ns):", dur)
	fmt.Println("Install time (ns):", stateInstallTime)
	fmt.Println("State size (bytes):", len(recvState))
	fmt.Println("Num of commands:", numCommands)
}

// AskForStateTransfer ...
func AskForStateTransfer(p, n string) ([]byte, uint64) {
	start := time.Now()
	recvState, err := sendStateRequest(p, n)
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

func validInterval(first, last string) error {
	if first == "" || last == "" {
		return fmt.Errorf("empty string informed: '%s' or '%s'", first, last)
	}

	p, err := strconv.ParseUint(first, 10, 64)
	if err != nil {
		return fmt.Errorf("could not parse '%s' as uint64", first)
	}

	n, err := strconv.ParseUint(last, 10, 64)
	if err != nil {
		return fmt.Errorf("could not parse '%s' as uint64", last)
	}

	if p > n {
		return fmt.Errorf("invalid interval [%s, %s] informed", first, last)
	}
	return nil
}
