package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"

	"github.com/hashicorp/raft"
)

// Server stores the state between every client
type Server struct {
	clients  []*Session
	joins    chan net.Conn
	incoming chan string
	outgoing chan string

	logger *log.Logger // The log of events for monitoring
	raft   *raft.Raft  // The consensus mechanism
}

// Broadcast sends a message to every other client on the room
func (svr *Server) Broadcast(data string) {
	for _, client := range svr.clients {
		client.outgoing <- data
	}
}

// Join threats a join requisition
func (svr *Server) Join(connection net.Conn) {
	client := NewSession(connection)
	svr.clients = append(svr.clients, client)
	go func() {
		for {
			svr.incoming <- <-client.incoming
		}
	}()
}

// Listen ...
func (svr *Server) Listen() {
	go func() {
		for {
			select {
			case data := <-svr.incoming:
				svr.Broadcast(data)
			case conn := <-svr.joins:
				svr.Join(conn)
			}
		}
	}()
}

// NewServer ...
func NewServer() *Server {
	svr := &Server{
		clients:  make([]*Session, 0),
		joins:    make(chan net.Conn),
		incoming: make(chan string),
		outgoing: make(chan string),
		logger:   log.New(os.Stderr, "[chatServer] ", log.LstdFlags),
	}

	svr.Listen()
	return svr
}

var svrPort string
var joinAddr string

func init() {
	flag.StringVar(&svrPort, "port", ":11000", "Set the chatRoom server bind address")
	flag.StringVar(&joinAddr, "join", "", "Set join address, if any")
}

func main() {

	flag.Parse()
	fmt.Println("Server Port:", svrPort)
	fmt.Println("Join addr:", joinAddr)

	chatRoom := NewServer()
	listener, err := net.Listen("tcp", svrPort)
	if err != nil {
		log.Fatalf("failed to start connection: %s", err.Error())
	}

	if joinAddr != "" {
		if err := join(joinAddr); err != nil {
			log.Fatalf("failed to join node at %s: %s", joinAddr, err.Error())
		}
	}

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				log.Fatalf("accept failed: %s", err.Error())
				continue
			}

			chatRoom.logger.Println("New client connected!")
			chatRoom.joins <- conn
		}
	}()

	terminate := make(chan os.Signal, 1)
	signal.Notify(terminate, os.Interrupt)
	<-terminate
}

func join(joinAddr string) error {
	return nil
}
