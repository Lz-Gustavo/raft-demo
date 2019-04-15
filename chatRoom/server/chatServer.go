package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
)

// Session struct which represents each ...
type Session struct {
	incoming chan string
	outgoing chan string
	reader   *bufio.Reader
	writer   *bufio.Writer
}

func (client *Session) Read() {
	for {
		line, err := client.reader.ReadString('\n')
		if (err == nil) && (len(line) > 1) {
			fmt.Println("Received message: ", line)
		}
		client.incoming <- line
	}
}

func (client *Session) Write() {
	for data := range client.outgoing {
		client.writer.WriteString(data)
		client.writer.Flush()
	}
}

// Listen ...
func (client *Session) Listen() {
	go client.Read()
	go client.Write()
}

// NewSession instantiates a new client
func NewSession(connection net.Conn) *Session {
	writer := bufio.NewWriter(connection)
	reader := bufio.NewReader(connection)

	client := &Session{
		incoming: make(chan string),
		outgoing: make(chan string),
		reader:   reader,
		writer:   writer,
	}

	client.Listen()

	return client
}

// Server stores the state between every client
type Server struct {
	clients  []*Session
	joins    chan net.Conn
	incoming chan string
	outgoing chan string
}

// Broadcast sends a message to every other client on the room
func (svr *Server) Broadcast(data string) {
	for _, client := range svr.clients {
		client.outgoing <- data
	}
}

// HandleRequest ...
func (svr *Server) HandleRequest(data string) {
	if strings.Contains(data, "get") {
		// ...
	} else if strings.Contains(data, "join") {
		// ...
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
			fmt.Println("New client connected!")
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
