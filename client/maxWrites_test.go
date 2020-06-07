package main

import (
	"encoding/binary"
	"log"
	"os"
	"testing"

	"raft-demo/beelog/pb"

	"github.com/golang/protobuf/proto"
)

const (
	numKeys  = 1000000
	execTime = 60
)

var testLogFilename = "/tmp/log-max-test.txt"

func TestMaxWritesByClientTime(b *testing.T) {

	storeValue = oneKB

	signal := make(chan bool)
	requests := make(chan *pb.Command, 512)
	logFile, err := os.OpenFile(testLogFilename, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalln(err)
	}

	go generateProtobufRequests(requests, signal, numKeys, storeValue)
	go killWorkers(execTime, signal)

	var numWrites uint64
	for {
		msg, ok := <-requests
		if !ok {
			break
		}

		serializedMessage, err := proto.Marshal(msg)
		if err != nil {
			log.Fatal(err)
		}

		command := &pb.Command{}
		err = proto.Unmarshal(serializedMessage, command)
		if err != nil {
			log.Fatal(err)
		}

		command.Id = 10
		serializedCmd, _ := proto.Marshal(command)
		binary.Write(logFile, binary.BigEndian, int32(len(serializedCmd)))
		_, err = logFile.Write(serializedCmd)
		if err != nil {
			log.Fatal(err)
		}

		numWrites++
	}

	b.Logf("Executed %d in %d seconds of experiment", numWrites, execTime)
}
