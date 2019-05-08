package main

import (
	"fmt"
	"net"
	"strings"
	"sync/atomic"
	"time"
)

// Server stores the state between every client
type Server struct {
	clients  []*Session
	joins    chan net.Conn
	incoming chan string

	req     uint64
	kvstore *Store
}

// NewServer constructs and starts a new Server
func NewServer(s *Store) *Server {
	svr := &Server{
		clients:  make([]*Session, 0),
		joins:    make(chan net.Conn),
		incoming: make(chan string),
		req:      0,
		kvstore:  s,
	}

	svr.Listen()
	svr.monitor()
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
	if validateReq(data) {
		if err := svr.kvstore.Propose(data, svr); err != nil {
			svr.kvstore.logger.Error(fmt.Sprintf("Failed to propose message: %q, error: %s\n", data, err.Error()))
		}
		atomic.AddUint64(&svr.req, 1)
	} else {
		svr.kvstore.logger.Warn(fmt.Sprintf("Operation: %q not recognized\n", data))
	}
}

// Join threats a join requisition from clients to the Server state
func (svr *Server) Join(connection net.Conn) {
	client := NewSession(connection)
	svr.clients = append(svr.clients, client)
	go func() {
		for {
			select {
			case data, ok := <-client.incoming:
				if !ok {
					return
				}
				svr.incoming <- data
			}
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

			case conn, ok := <-svr.joins:
				if !ok {
					return
				}
				svr.Join(conn)
			}
		}
	}()
}

func (svr *Server) monitor() {
	go func() {
		for {
			time.Sleep(1 * time.Second)
			cont := atomic.SwapUint64(&svr.req, 0)
			svr.kvstore.logger.Info(fmt.Sprintf("Thoughput(cmds/s): %d", cont))
		}
	}()
}

func validateReq(requisition string) bool {

	requisition = strings.ToLower(requisition)
	splited := strings.Split(requisition, "-")

	if splited[0] == "set" {
		return len(splited) >= 3
	} else if splited[0] == "get" || splited[0] == "delete" {
		return len(splited) >= 2
	} else {
		return false
	}
}
