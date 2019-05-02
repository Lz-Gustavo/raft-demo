package main

import (
	"net"
	"strings"
)

// Server stores the state between every client
type Server struct {
	clients  []*Session
	joins    chan net.Conn
	incoming chan string

	kvstore *Store
}

// NewServer constructs and starts a new Server
func NewServer(s *Store) *Server {
	svr := &Server{
		clients:  make([]*Session, 0),
		joins:    make(chan net.Conn),
		incoming: make(chan string),
		kvstore:  s,
	}

	svr.Listen()
	return svr
}

// Exit closes the raft context and releases any resources allocated
func (svr *Server) Exit() {

	svr.kvstore.raft.Shutdown()
	for _, v := range svr.clients {
		v.Disconnect()
	}
	close(svr.joins)
	close(svr.incoming)
}

// Broadcast sends a message to every other client on the room
func (svr *Server) Broadcast(data string) {
	for _, client := range svr.clients {
		client.outgoing <- data
	}
}

// HandleRequest handles the client requistion, checking if it matches the right syntax
// before proposing it to the FSM
func (svr *Server) HandleRequest(data string) {

	lowerCase := strings.ToLower(data)
	if validateReq(lowerCase) {

		if strings.HasPrefix(lowerCase, "get") {
			// TODO: Respond get value

		}
		if err := svr.kvstore.Propose(data); err != nil {
			svr.kvstore.logger.Printf("Failed to propose message: %q, error: %s\n", data, err.Error())
		}
	} else {
		svr.kvstore.logger.Printf("Operation: %q not recognized\n", data)
	}
}

// Join threats a join requisition from clients to the Server state
func (svr *Server) Join(connection net.Conn) {
	client := NewSession(connection)
	svr.clients = append(svr.clients, client)
	go func() {
		for {
			svr.incoming <- <-client.incoming
		}
	}()
}

// Listen receives incoming messagens and new connections from clients
func (svr *Server) Listen() {
	go func() {
		for {
			select {
			case data, ok := <-svr.incoming:
				if !ok {
					return
				}
				svr.HandleRequest(data)

			case conn := <-svr.joins:
				svr.Join(conn)
			}
		}
	}()
}

// TODO:
func validateReq(requisition string) bool {
	return true
}
