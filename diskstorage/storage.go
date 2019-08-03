package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Lz-Gustavo/journey"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/raft"
)

const (
	oneTweet = 128
	oneKB    = 1024
	fourKB   = 4096

	// TODO: Assign this value as a command line argument
	storeValuesOffset = oneTweet
)

const (
	retainSnapshotCount = 2
	raftTimeout         = 10 * time.Second
	logLevel            = "ERROR"

	fileBatchSync = 100
)

// Custom configuration over default for testing
func configRaft() *raft.Config {

	config := raft.DefaultConfig()
	config.SnapshotInterval = 24 * time.Hour
	config.SnapshotThreshold = 2 << 62
	config.LogLevel = logLevel
	return config
}

// Store is a simple key-value store, where all changes are made via Raft consensus.
type Store struct {
	RaftDir  string
	RaftBind string

	raft   *raft.Raft
	logger hclog.Logger

	Logging bool
	recov   *journey.Log

	Local      *os.File
	valueSize  int
	storeValue []byte

	batchSync  uint32
	writeCount uint32
}

// New returns a new Store.
func New(storeFilename string) (*Store, error) {

	s := &Store{
		batchSync: fileBatchSync,
		valueSize: storeValuesOffset,
		logger: hclog.New(&hclog.LoggerOptions{
			Name:   "store",
			Level:  hclog.LevelFromString(logLevel),
			Output: os.Stderr,
		}),
	}
	s.storeValue = []byte(strings.Repeat("@", s.valueSize))

	var err error
	if _, exists := os.Stat(storeFilename); exists == nil {
		s.Local, err = os.OpenFile(storeFilename, os.O_RDWR, 0644)
	} else if os.IsNotExist(exists) {
		s.Local, err = os.Create(storeFilename)
	}
	if err != nil {
		return nil, err
	}

	if joinHandlerAddr != "" {
		s.ListenRaftJoins(joinHandlerAddr)
	}

	if *logfolder != "" {
		s.Logging = true
		config := journey.DefaultConfig
		config.Batch = 100
		config.Class = journey.Serialized
		s.recov = journey.New(config, *logfolder+"log-file-"+svrID+".txt")
	}
	return s, nil
}

// Propose invokes Raft.Apply to propose a new command following protocol's atomic broadcast
// to the application's FSM. Sends an "OK" repply to inform commitment. This procedure applies
// "Get" requisitions to prevent inconsistent reads (that do not follow total ordering). etcd's
// issue #741 gives a good explanation about this problem.
func (s *Store) Propose(msg []byte, svr *Server, clientIP string) error {

	if s.raft.State() != raft.Leader {
		return nil
	}

	f := s.raft.Apply(msg, raftTimeout)
	err := f.Error()
	if err != nil {
		return err
	}

	switch f.Response().(type) {
	case string:
		response := strings.Split(f.Response().(string), "-")
		udpAddr := strings.Join([]string{clientIP, ":", response[0]}, "")
		clientRepply := strings.Join([]string{"OK: ", response[1], "\n"}, "")
		svr.SendUDP(udpAddr, clientRepply)
		break

	default:
		return fmt.Errorf("Unrecognized data response %q", f.Response())
	}
	return nil
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

	s.logger.Debug(fmt.Sprintf("received join request for remote node %s at %s", nodeID, addr))
	configFuture := s.raft.GetConfiguration()
	if err := configFuture.Error(); err != nil {
		s.logger.Error(fmt.Sprintf("failed to get raft configuration: %v", err))
		return err
	}

	for _, rep := range configFuture.Configuration().Servers {

		// If a node already exists with either the joining node's ID or address,
		// that node may need to be removed from the config first.
		if rep.ID == raft.ServerID(nodeID) || rep.Address == raft.ServerAddress(addr) {

			// However if *both* the ID and the address are the same, then nothing -- not even
			// a join operation -- is needed.
			if rep.Address == raft.ServerAddress(addr) && rep.ID == raft.ServerID(nodeID) {
				s.logger.Debug(fmt.Sprintf("node %s at %s already member of cluster, ignoring join request", nodeID, addr))
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

	s.logger.Debug(fmt.Sprintf("node %s at %s joined successfully", nodeID, addr))
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
