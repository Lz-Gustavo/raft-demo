package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	bl "raft-demo/beelog"
	"raft-demo/beelog/pb"

	"github.com/golang/protobuf/proto"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/raft"
)

const (
	retainSnapshotCount = 2
	raftTimeout         = 10 * time.Second
	logLevel            = "INFO"
	compressValues      = false

	preInitialize = true
	numInitKeys   = 1000000
	initValueSize = 1024

	// Used in catastrophic fault models, where crash faults must be recoverable even if
	// all nodes presented in the consensus cluster are down. Always set to false in any
	// other cases, because this strong assumption greatly degradates performance.
	catastrophicFaults = false

	// TODO: describe ...
	beelogTest    = true
	inMemStateLog = true
	reduceAlg     = bl.IterDFSAvl
)

var (
	initValue = []byte(strings.Repeat("!", initValueSize))
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
	inMem    bool

	mu sync.Mutex
	m  map[string][]byte

	raft   *raft.Raft
	logger hclog.Logger

	Logging  bool
	LogFile  *os.File
	inMemLog *[]pb.Command
	avl      *bl.AVLTreeHT

	compress   bool
	gzipBuffer bytes.Buffer
}

// New returns a new Store.
func New(ctx context.Context, inMem bool) *Store {
	s := &Store{
		m:        make(map[string][]byte),
		inMem:    inMem,
		compress: compressValues,
		logger: hclog.New(&hclog.LoggerOptions{
			Name:   "store",
			Level:  hclog.LevelFromString(logLevel),
			Output: os.Stderr,
		}),
	}

	if joinHandlerAddr != "" {
		go s.ListenRaftJoins(ctx, joinHandlerAddr)
	}

	if recovHandlerAddr != "" {
		go s.ListenStateTransfer(ctx, recovHandlerAddr)
	}

	if *logfolder != "" {
		s.Logging = true
		logFileName := *logfolder + "log-file-" + svrID + ".txt"
		s.LogFile = createFile(logFileName)

	} else {
		s.Logging = inMemStateLog || beelogTest
		if beelogTest {
			s.avl = bl.NewAVLTreeHT()
		} else {
			s.inMemLog = &[]pb.Command{}
		}
	}

	if compressValues {
		s.gzipBuffer.Reset()
		wtr := gzip.NewWriter(&s.gzipBuffer)
		wtr.Write([]byte(initValue))

		if err := wtr.Flush(); err != nil {
			log.Fatalln(err)
		}
		if err := wtr.Close(); err != nil {
			log.Fatalln(err)
		}
		initValue = s.gzipBuffer.Bytes()
	}

	if preInitialize {
		for i := 0; i < numInitKeys; i++ {
			s.m[strconv.Itoa(i)] = initValue
		}
	}
	return s
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

// testGet returns the value for the given key, just using in unit tests since it results
// in an inconsistence read operation, not following total ordering.
func (s *Store) testGet(key string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return string(s.m[key])
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
	dir := "checkpoints/" + localID
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
func (s *Store) ListenRaftJoins(ctx context.Context, addr string) {

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("failed to bind connection at %s: %s", addr, err.Error())
	}

	for {
		select {
		case <-ctx.Done():
			return

		default:
			conn, err := listener.Accept()
			if err != nil {
				log.Fatalf("accept failed: %s", err.Error())
			}

			request, _ := bufio.NewReader(conn).ReadString('\n')

			data := strings.Split(request, "-")
			if len(data) < 3 {
				log.Fatalf("incorrect join request, got: %s", data)
			}

			data[2] = strings.TrimSuffix(data[2], "\n")
			voter, _ := strconv.ParseBool(data[2])
			err = s.JoinRaft(data[0], data[1], voter)
			if err != nil {
				log.Fatalf("failed to join node at %s: %s", data[1], err.Error())
			}
		}
	}
}

// UnsafeStateRecover ...
func (s *Store) UnsafeStateRecover(p, n uint64, activePipe net.Conn) error {
	if !s.Logging {
		return fmt.Errorf("cannot retrieve application-level log from a non-logged application")
	}

	var (
		logs []byte
		err  error
		rd   io.Reader
		buff *bytes.Buffer
	)

	if !(inMemStateLog || beelogTest) {
		// persistent stored log, currently returns the entire log ignoring [p, n] interval

		logFileName := *logfolder + "log-file-" + svrID + ".txt"
		fd, _ := os.OpenFile(logFileName, os.O_RDONLY, 0644)
		defer fd.Close()
		rd = fd

	} else {
		var cmds []pb.Command
		if beelogTest {
			cmds, err = bl.ApplyReduceAlgo(s.avl, reduceAlg, p, n)
			if err != nil {
				return err
			}

		} else {
			// local in-mem log
			for _, c := range *s.inMemLog {
				if c.Id >= p && c.Id <= n {
					cmds = append(cmds, c)
				}
			}
		}

		buff = bytes.NewBuffer(nil)
		for _, c := range cmds {
			raw, err := proto.Marshal(&c)
			if err != nil {
				return err
			}

			// writing size of each serialized message as streaming delimiter
			err = binary.Write(buff, binary.BigEndian, int32(len(raw)))
			if err != nil {
				return err
			}

			_, err = buff.Write(raw)
			if err != nil {
				return err
			}
		}
		rd = buff
	}

	logs, err = ioutil.ReadAll(rd)
	if err != nil {
		return err
	}

	signalError := make(chan error, 0)
	go func(dataToSend []byte, pipe net.Conn, signal chan<- error) {

		_, err := pipe.Write(dataToSend)
		signal <- err

	}(logs, activePipe, signalError)
	return <-signalError
}

// ListenStateTransfer ...
func (s *Store) ListenStateTransfer(ctx context.Context, addr string) {

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("failed to bind connection at %s: %s", addr, err.Error())
	}

	for {
		select {
		case <-ctx.Done():
			return

		default:
			conn, err := listener.Accept()
			if err != nil {
				log.Fatalf("accept failed: %s", err.Error())
			}

			request, _ := bufio.NewReader(conn).ReadString('\n')

			data := strings.Split(request, "-")
			if len(data) != 3 {
				log.Fatalf("incorrect state request, got: %s", data)
			}

			data[2] = strings.TrimSuffix(data[2], "\n")
			firstIndex, _ := strconv.Atoi(data[1])
			lastIndex, _ := strconv.Atoi(data[2])

			err = s.UnsafeStateRecover(uint64(firstIndex), uint64(lastIndex), conn)
			if err != nil {
				log.Fatalf("failed to transfer log to node located at %s: %s", data[0], err.Error())
			}

			err = conn.Close()
			if err != nil {
				log.Fatalf("Error encountered on connection close: %s", err.Error())
			}
		}
	}
}

func createFile(filename string) *os.File {

	var flags int
	if catastrophicFaults {
		flags = os.O_SYNC | os.O_WRONLY
	} else {
		flags = os.O_WRONLY
	}

	var fd *os.File
	if _, exists := os.Stat(filename); exists == nil {
		fd, _ = os.OpenFile(filename, flags, 0644)
	} else if os.IsNotExist(exists) {
		fd, _ = os.OpenFile(filename, os.O_CREATE|flags, 0644)
	} else {
		log.Fatalln("Could not create file", filename, ":", exists.Error())
		return nil
	}
	return fd
}
