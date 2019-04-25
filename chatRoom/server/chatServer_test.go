package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"testing"
)

func TestServerStart(t *testing.T) {

	// Initialize RaftNode0
	go func() {

		svrPort := ":11000"
		svrID := "node0"

		chatRoom := NewServerListenRJ(":13000")
		listener, err := net.Listen("tcp", svrPort)
		if err != nil {
			log.Fatalf("failed to start connection: %s", err.Error())
		}

		if err := chatRoom.StartRaft(true, svrID, ":12000"); err != nil {
			log.Fatalf("failed to start raft cluster: %s", err.Error())
		}

		for {
			conn, err := listener.Accept()
			if err != nil {
				log.Fatalf("accept failed: %s", err.Error())
			}
			chatRoom.joins <- conn
		}
	}()

	// Initialize RaftNode1
	go func() {

		svrID := "node1"
		svrPort := ":11001"
		raftAddr := ":12001"
		joinAddr := ":13000"

		chatRoom := NewServer()
		listener, err := net.Listen("tcp", svrPort)
		if err != nil {
			log.Fatalf("failed to start connection: %s", err.Error())
		}

		if err := chatRoom.StartRaft(false, svrID, raftAddr); err != nil {
			log.Fatalf("failed to start raft cluster: %s", err.Error())
		}

		// Send a join connection to the first node
		joinConn, err := net.Dial("tcp", joinAddr)
		if err != nil {
			log.Fatalf("failed to connect to leader node at %s: %s", joinAddr, err.Error())
		}

		_, err = fmt.Fprint(joinConn, svrID+"-"+raftAddr+"-"+"true"+"\n")
		if err != nil {
			log.Fatalf("failed to send join request to node at %s: %s", joinAddr, err.Error())
		}
		joinConn.Close()

		for {
			conn, err := listener.Accept()
			if err != nil {
				log.Fatalf("accept failed: %s", err.Error())
			}
			chatRoom.joins <- conn
		}
	}()

	// Initialize RaftNode2
	go func() {

		svrID := "node2"
		svrPort := ":11002"
		raftAddr := ":12002"
		joinAddr := ":13000"

		chatRoom := NewServer()
		listener, err := net.Listen("tcp", svrPort)
		if err != nil {
			log.Fatalf("failed to start connection: %s", err.Error())
		}

		if err := chatRoom.StartRaft(false, svrID, raftAddr); err != nil {
			log.Fatalf("failed to start raft cluster: %s", err.Error())
		}

		// Send a join connection to the first node
		joinConn, err := net.Dial("tcp", joinAddr)
		if err != nil {
			log.Fatalf("failed to connect to leader node at %s: %s", joinAddr, err.Error())
		}

		_, err = fmt.Fprint(joinConn, svrID+"-"+raftAddr+"-"+"true"+"\n")
		if err != nil {
			log.Fatalf("failed to send join request to node at %s: %s", joinAddr, err.Error())
		}
		joinConn.Close()

		for {
			conn, err := listener.Accept()
			if err != nil {
				log.Fatalf("accept failed: %s", err.Error())
			}
			chatRoom.joins <- conn
		}
	}()

	terminate := make(chan os.Signal, 1)
	signal.Notify(terminate, os.Interrupt)
	<-terminate
}
