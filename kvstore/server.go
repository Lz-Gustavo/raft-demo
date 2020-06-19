package main

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"sync/atomic"
	"time"
)

// Server stores the state between every client
type Server struct {
	clients  []*Session
	joins    chan net.Conn
	incoming chan *Request

	t          *time.Timer
	req        uint64
	throughput *os.File
	kvstore    *Store
}

// NewServer constructs and starts a new Server
func NewServer(ctx context.Context, s *Store) *Server {
	svr := &Server{
		clients:  make([]*Session, 0),
		joins:    make(chan net.Conn),
		incoming: make(chan *Request),
		req:      0,
		kvstore:  s,
		t:        time.NewTimer(time.Second),
	}
	svr.throughput = createFile(svrID + "-throughput.out")

	go svr.Listen(ctx)
	go svr.monitor(ctx)
	return svr
}

// Exit closes the raft context and releases any resources allocated
func (svr *Server) Exit() {

	svr.kvstore.raft.Shutdown()
	if svr.kvstore.Logging == DiskTrad {
		svr.kvstore.LogFile.Close()
	}
	for _, v := range svr.clients {
		v.Disconnect()
	}
}

// Broadcast sends a message to every other client on the room
func (svr *Server) Broadcast(data string) {
	for _, client := range svr.clients {
		client.outgoing <- data
	}
}

// SendUDP sends a UDP repply to a client listening on 'addr'
func (svr *Server) SendUDP(addr string, message string) {
	conn, _ := net.Dial("udp", addr)
	defer conn.Close()
	conn.Write([]byte(message))
}

// HandleRequest handles the client requistion, checking if it matches the right syntax
// before proposing it to the FSM
func (svr *Server) HandleRequest(cmd *Request) {

	data := bytes.TrimSuffix(cmd.Command, []byte("\n"))
	if err := svr.kvstore.Propose(data, svr, cmd.Ip); err != nil {
		svr.kvstore.logger.Error(fmt.Sprintf("Failed to propose message: %q, error: %s\n", data, err.Error()))
	}
	atomic.AddUint64(&svr.req, 1)
}

// Join threats a join requisition from clients to the Server state
func (svr *Server) Join(ctx context.Context, connection net.Conn) {
	client := NewSession(connection)
	svr.clients = append(svr.clients, client)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return

			case data := <-client.incoming:
				svr.incoming <- data
			}
		}
	}()
}

// Listen receives incoming messagens and new connections from clients
func (svr *Server) Listen(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return

		case data := <-svr.incoming:
			svr.HandleRequest(data)

		case conn := <-svr.joins:
			svr.Join(ctx, conn)
		}
	}
}

func (svr *Server) monitor(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return

		case <-svr.t.C:
			cont := atomic.SwapUint64(&svr.req, 0)
			//svr.kvstore.logger.Info(fmt.Sprintf("Thoughput(cmds/s): %d", cont))
			svr.throughput.WriteString(fmt.Sprintf("%v\n", cont))
			svr.t.Reset(time.Second)
		}
	}
}

// Legacy code, used only on ad-hoc message formats.
func validateReq(requisition string) bool {

	requisition = strings.ToLower(requisition)
	splited := strings.Split(requisition, "-")

	if splited[1] == "set" {
		return len(splited) >= 3
	} else if splited[1] == "get" || splited[1] == "delete" {
		return len(splited) >= 2
	} else {
		return false
	}
}
