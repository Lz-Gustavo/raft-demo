package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"journey"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Lz-Gustavo/raft"
)

const (
	retainSnapshotCount = 2
	raftTimeout         = 10 * time.Second
)

// Custom configuration over default for testing
func configRaft() *raft.Config {

	config := raft.DefaultConfig()
	config.SnapshotInterval = 5 * time.Minute
	config.SnapshotThreshold = 1000024
	config.LogLevel = "WARN"

	return config
}

type command struct {
	Op    string `json:"op,omitempty"`
	Key   string `json:"key,omitempty"`
	Value string `json:"value,omitempty"`
}

// Store is a simple key-value store, where all changes are made via Raft consensus.
type Store struct {
	RaftDir  string
	RaftBind string
	inmem    bool

	mu sync.Mutex
	m  map[string]string

	raft   *raft.Raft
	logger *log.Logger
	recov  *journey.Log
}

// New returns a new Store.
func New(inmem bool) *Store {

	s := &Store{
		m:      make(map[string]string),
		inmem:  inmem,
		logger: log.New(os.Stderr, "[store] ", log.LstdFlags),
		recov:  journey.New("log-file.txt"),
	}

	if joinHandlerAddr != "" {
		s.ListenRaftJoins(joinHandlerAddr)
	}
	return s
}

// Get returns the value for the given key.
func (s *Store) Get(key string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	str := fmt.Sprintf("Get key: %s", key)
	s.recov.Put(0, journey.Read, str, time.Now().Format(time.Stamp))

	return s.m[key], nil
}

// Set sets the value for the given key.
func (s *Store) Set(key, value string) error {
	if s.raft.State() != raft.Leader {
		return fmt.Errorf("not leader")
	}

	c := &command{
		Op:    "set",
		Key:   key,
		Value: value,
	}
	b, err := json.Marshal(c)
	if err != nil {
		return err
	}

	f := s.raft.Apply(b, raftTimeout)
	if f.Error() == nil {
		str := fmt.Sprintf("Set key: %s, value: %s", key, value)
		s.recov.Put(0, journey.Write, str, time.Now().Format(time.Stamp))
	}

	return f.Error()
}

// Delete deletes the given key.
func (s *Store) Delete(key string) error {
	if s.raft.State() != raft.Leader {
		return fmt.Errorf("not leader")
	}

	c := &command{
		Op:  "delete",
		Key: key,
	}
	b, err := json.Marshal(c)
	if err != nil {
		return err
	}

	f := s.raft.Apply(b, raftTimeout)
	if f.Error() == nil {
		str := fmt.Sprintf("Delete key: %s", key)
		s.recov.Put(0, journey.Write, str, time.Now().Format(time.Stamp))
	}

	return f.Error()
}

// StartRaft opens the store. If enableSingle is set, and there are no existing peers,
// then this node becomes the first node, and therefore leader, of the cluster.
// localID should be the server identifier for this node.
func (s *Store) StartRaft(enableSingle bool, localID string, localRaftAddr string) error {

	// Setup Raft configuration.
	config := configRaft()
	config.LocalID = raft.ServerID(localID)

	// Setup Raft communication.
	addr, err := net.ResolveTCPAddr("tcp", localRaftAddr)
	if err != nil {
		return err
	}
	transport, err := raft.NewTCPTransport(localRaftAddr, addr, 3, 10*time.Second, os.Stderr)
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
	ra, err := raft.NewRaft(config, (*fsm)(s), logStore, stableStore, snapshots, transport)
	if err != nil {
		return fmt.Errorf("new raft: %s", err)
	}
	s.raft = ra

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

// JoinRaft joins a raft node, identified by nodeID and located at addr
func (s *Store) JoinRaft(nodeID, addr string, voter bool) error {

	s.logger.Printf("received join request for remote node %s at %s", nodeID, addr)
	configFuture := s.raft.GetConfiguration()
	if err := configFuture.Error(); err != nil {
		s.logger.Printf("failed to get raft configuration: %v", err)
		return err
	}

	for _, rep := range configFuture.Configuration().Servers {

		// If a node already exists with either the joining node's ID or address,
		// that node may need to be removed from the config first.
		if rep.ID == raft.ServerID(nodeID) || rep.Address == raft.ServerAddress(addr) {

			// However if *both* the ID and the address are the same, then nothing -- not even
			// a join operation -- is needed.
			if rep.Address == raft.ServerAddress(addr) && rep.ID == raft.ServerID(nodeID) {
				s.logger.Printf("node %s at %s already member of cluster, ignoring join request", nodeID, addr)
				return nil
			}

			future := s.raft.RemoveServer(rep.ID, 0, 0)
			if err := future.Error(); err != nil {
				return fmt.Errorf("error removing existing node %s at %s: %s", nodeID, addr, err)
			}
		}
	}

	if voter {
		f := s.raft.AddVoter(raft.ServerID(nodeID), raft.ServerAddress(addr), 0, 0)
		if f.Error() != nil {
			return f.Error()
		}
	} else {
		f := s.raft.AddNonvoter(raft.ServerID(nodeID), raft.ServerAddress(addr), 0, 0)
		if f.Error() != nil {
			return f.Error()
		}
	}

	s.logger.Printf("node %s at %s joined successfully", nodeID, addr)
	return nil
}

// ListenRaftJoins receives incoming join requests to the raft cluster. Its initialized
// when "-hjoin" flag is specified, and it can be set only in the first node in case you
// have a static/imutable cluster architecture
func (s *Store) ListenRaftJoins(addr string) {

	go func() {
		listener, err := net.Listen("tcp", addr)
		if err != nil {
			log.Fatalf("failed to bind connection at %s: %s", addr, err.Error())
		}

		for {
			conn, err := listener.Accept()
			if err != nil {
				log.Fatalf("accept failed: %s", err.Error())
			}

			request, _ := bufio.NewReader(conn).ReadString('\n')

			data := strings.Split(request, "-")
			if len(data) < 3 {
				log.Fatalf("incorrect join request, data: %s", data)
			}

			data[2] = strings.TrimSuffix(data[2], "\n")
			voter, _ := strconv.ParseBool(data[2])
			err = s.JoinRaft(data[0], data[1], voter)
			if err != nil {
				log.Fatalf("failed to join node at %s: %s", data[1], err.Error())
			}
		}
	}()
}
