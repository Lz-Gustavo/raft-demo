package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
)

var svrID string
var svrPort string
var joinAddr string
var raftAddr string
var joinHandlerAddr string

func init() {
	flag.StringVar(&svrID, "id", "", "Set server unique ID")
	flag.StringVar(&svrPort, "port", ":11000", "Set the server bind address")
	flag.StringVar(&raftAddr, "raft", ":12000", "Set RAFT consensus bind address")
	flag.StringVar(&joinHandlerAddr, "hjoin", "", "Set port id to receive join requests on the raft cluster")
	flag.StringVar(&joinAddr, "join", "", "Set join address, if any")
}

func main() {

	flag.Parse()
	if svrID == "" {
		log.Fatalln("must set a server ID, run with: ./server -id 'svrID'")
	}

	fmt.Println("Server ID:", svrID)
	fmt.Println("Server Port:", svrPort)
	fmt.Println("Raft Port:", raftAddr)
	fmt.Println("Raft JoinAcceptor:", joinHandlerAddr)
	fmt.Println("Join addr:", joinAddr)

	// Initialize the Chat Server
	kvs := New(true)
	listener, err := net.Listen("tcp", svrPort)
	if err != nil {
		log.Fatalf("failed to start connection: %s", err.Error())
	}

	// Start the Raft cluster
	if err := kvs.StartRaft(joinAddr == "", svrID, raftAddr); err != nil {
		log.Fatalf("failed to start raft cluster: %s", err.Error())
	}

	server := NewServer(kvs)

	// Send a join request, if any
	if joinAddr != "" {
		joinConn, err := net.Dial("tcp", joinAddr)
		if err != nil {
			log.Fatalf("failed to connect to leader node at %s: %s", joinAddr, err.Error())
		}

		_, err = fmt.Fprint(joinConn, svrID+"-"+raftAddr+"-"+"true"+"\n")
		if err != nil {
			log.Fatalf("failed to send join request to node at %s: %s", joinAddr, err.Error())
		}
		joinConn.Close()
	}

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				log.Fatalf("accept failed: %s", err.Error())
			}

			//server.logger.Println("New client connected!")
			server.joins <- conn
		}
	}()

	terminate := make(chan os.Signal, 1)
	signal.Notify(terminate, os.Interrupt)
	<-terminate

	//server.Shutdown()
}
