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
	"time"

	"github.com/hashicorp/raft"
)

const (
	raftTimeout = 10 * time.Second
)

// Server stores the state between every client
type Server struct {
	clients  []*Session
	joins    chan net.Conn
	incoming chan string

	logger      *log.Logger // Log of events for monitoring purposes
	raft        *raft.Raft  // Instance of the raft consensus protocol
	clusterJoin chan string
}

// NewServer ...
func NewServer() *Server {
	svr := &Server{
		clients:  make([]*Session, 0),
		joins:    make(chan net.Conn),
		incoming: make(chan string),

		logger:      log.New(os.Stderr, "[chatServer] ", log.LstdFlags),
		clusterJoin: make(chan string),
	}

	svr.Listen()
	if joinHandlerAddr != "" {
		svr.ListenRaftJoins()
	}
	return svr
}

// Broadcast sends a message to every other client on the room
func (svr *Server) Broadcast(data string) {
	for _, client := range svr.clients {
		client.outgoing <- data
	}

	// Propose the value to the consensus mechanism
	if svr.raft.State() == raft.Leader {
		svr.raft.Apply([]byte(data), raftTimeout)
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

// ListenRaftJoins ...
func (svr *Server) ListenRaftJoins() {

	go func() {

		listener, err := net.Listen("tcp", joinHandlerAddr)
		if err != nil {
			log.Fatalf("failed to bind connection at %s: %s", joinHandlerAddr, err.Error())
		}

		for {
			conn, err := listener.Accept()
			if err != nil {
				log.Fatalf("accept failed: %s", err.Error())
			}

			request, _ := bufio.NewReader(conn).ReadString('\n')

			data := strings.Split(request, "-")
			err = svr.JoinRaft(data[0], data[1])
			if err != nil {
				log.Fatalf("failed to join node at %s: %s", data[1], err.Error())
			}
		}
	}()
}

// StartRaft initializes the node to be part of the raft cluster
func (svr *Server) StartRaft(enableSingle bool, localID string) error {

	// Setup Raft configuration.
	config := raft.DefaultConfig()
	config.LocalID = raft.ServerID(localID)

	// Setup Raft communication.
	addr, err := net.ResolveTCPAddr("tcp", raftAddr)
	if err != nil {
		return err
	}
	transport, err := raft.NewTCPTransport(raftAddr, addr, 3, 10*time.Second, os.Stderr)
	if err != nil {
		return err
	}

	// Using just in-memory storage (could use boltDB in the key-value application)
	logStore := raft.NewInmemStore()
	stableStore := raft.NewInmemStore()

	// Create a fake snapshot store
	dir := "checkpoints/" + svrID
	snapshots, err := raft.NewFileSnapshotStore(dir, 2, os.Stderr)
	if err != nil {
		return fmt.Errorf("file snapshot store: %s", err)
	}

	// Instantiate the Raft systems.
	ra, err := raft.NewRaft(config, (*fsm)(svr), logStore, stableStore, snapshots, transport)
	if err != nil {
		return fmt.Errorf("new raft: %s", err)
	}
	svr.raft = ra

	if enableSingle {
		configuration := raft.Configuration{
			Servers: []raft.Server{
				{
					ID:      config.LocalID,
					Address: transport.LocalAddr(),
				},
			},
		}
		ra.BootstrapCluster(configuration)
	}

	return nil
}

// JoinRaft joins a node, identified by nodeID and located at addr
func (svr *Server) JoinRaft(nodeID, addr string) error {

	addr = strings.TrimSuffix(addr, "\n")
	svr.logger.Printf("received join request for remote node %s at %s", nodeID, addr)
	configFuture := svr.raft.GetConfiguration()
	if err := configFuture.Error(); err != nil {
		svr.logger.Printf("failed to get raft configuration: %v", err)
		return err
	}

	for _, rep := range configFuture.Configuration().Servers {

		// If a node already exists with either the joining node's ID or address,
		// that node may need to be removed from the config first.
		if rep.ID == raft.ServerID(nodeID) || rep.Address == raft.ServerAddress(addr) {

			// However if *both* the ID and the address are the same, then nothing -- not even
			// a join operation -- is needed.
			if rep.Address == raft.ServerAddress(addr) && rep.ID == raft.ServerID(nodeID) {
				svr.logger.Printf("node %s at %s already member of cluster, ignoring join request", nodeID, addr)
				return nil
			}

			future := svr.raft.RemoveServer(rep.ID, 0, 0)
			if err := future.Error(); err != nil {
				return fmt.Errorf("error removing existing node %s at %s: %s", nodeID, addr, err)
			}
		}
	}

	f := svr.raft.AddVoter(raft.ServerID(nodeID), raft.ServerAddress(addr), 0, 0)
	if f.Error() != nil {
		return f.Error()
	}

	svr.logger.Printf("node %s at %s joined successfully", nodeID, addr)
	return nil
}

var svrID string
var svrPort string
var joinAddr string
var raftAddr string
var joinHandlerAddr string

func init() {
	flag.StringVar(&svrID, "id", "", "Set server unique ID")
	flag.StringVar(&svrPort, "port", ":11000", "Set the chatRoom server bind address")
	flag.StringVar(&raftAddr, "raft", ":12000", "Set RAFT consensus bind address")
	flag.StringVar(&joinHandlerAddr, "hjoin", "", "Set port id to receive join requests on the raft cluster")
	flag.StringVar(&joinAddr, "join", "", "Set join address, if any")
}

func main() {

	flag.Parse()
	fmt.Println("Server ID:", svrID)
	fmt.Println("Server Port:", svrPort)
	fmt.Println("Raft Port:", raftAddr)
	fmt.Println("Raft JoinAcceptor:", joinHandlerAddr)
	fmt.Println("Join addr:", joinAddr)

	// Initialize the Chat Server
	chatRoom := NewServer()
	listener, err := net.Listen("tcp", svrPort)
	if err != nil {
		log.Fatalf("failed to start connection: %s", err.Error())
	}

	// Start the Raft cluster
	if err := chatRoom.StartRaft(joinAddr == "", svrID); err != nil {
		log.Fatalf("failed to start raft cluster: %s", err.Error())
	}

	// Send a join request, if any
	if joinAddr != "" {
		joinConn, err := net.Dial("tcp", joinAddr)
		if err != nil {
			log.Fatalf("failed to connect to leader node at %s: %s", joinAddr, err.Error())
		}

		_, err = fmt.Fprint(joinConn, svrID+"-"+raftAddr+"\n")
		if err != nil {
			log.Fatalf("failed to send join request to node at %s: %s", joinAddr, err.Error())
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
