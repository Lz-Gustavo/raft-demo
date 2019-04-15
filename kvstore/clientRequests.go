package main

import (
	"bufio"
	"net"
	"raft-demo/kvstore/store"
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
		line, _ := client.reader.ReadString('\n')
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

	kvstore *store.Store
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

	} else if strings.Contains(data, "join") {

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
				svr.HandleRequest(data)
			case conn := <-svr.joins:
				svr.Join(conn)
			}
		}
	}()
}

// NewServer ...
func NewServer(s *store.Store) *Server {
	svr := &Server{
		clients:  make([]*Session, 0),
		joins:    make(chan net.Conn),
		incoming: make(chan string),
		outgoing: make(chan string),
		kvstore:  s,
	}

	svr.Listen()
	return svr
}
