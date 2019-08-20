package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"time"

	"github.com/hashicorp/raft"
)

const (
	// Used in catastrophic fault models, where crash faults must be recoverable even if
	// all nodes presented in the consensus cluster are down. Always set to false in any
	// other cases, because this strong assumption greatly degradates performance.
	catastrophicFaults = true
)

// Custom configuration over default for testing
func configRaft() *raft.Config {

	config := raft.DefaultConfig()
	config.SnapshotInterval = 24 * time.Hour
	config.SnapshotThreshold = 2 << 62
	config.LogLevel = "WARN"
	return config
}

// Logger struct represents the Logger process state. Member of the Raft cluster as a
// non-Voter participant and thus, just recording proposed commands to the FSM
type Logger struct {
	log     *log.Logger
	raft    *raft.Raft
	LogFile *os.File
}

// NewLogger constructs a new Logger struct and its dependencies
func NewLogger() *Logger {

	l := &Logger{
		log: log.New(os.Stderr, "[chatLogger] ", log.LstdFlags),
	}

	var flags int
	if catastrophicFaults {
		flags = os.O_CREATE | os.O_EXCL | os.O_APPEND | os.O_SYNC
	} else {
		flags = os.O_CREATE | os.O_EXCL | os.O_APPEND
	}

	logFileName := *logfolder + "log-file-" + logID + ".txt"
	if catastrophicFaults {
		flags = os.O_SYNC | os.O_WRONLY
	} else {
		flags = os.O_WRONLY
	}

	if _, exists := os.Stat(logFileName); exists == nil {
		l.LogFile, _ = os.OpenFile(logFileName, flags, 0644)
	} else if os.IsNotExist(exists) {
		l.LogFile, _ = os.OpenFile(logFileName, os.O_CREATE|flags, 0644)
	} else {
		log.Fatalln("Could not create log file:", exists.Error())
	}

	return l
}

// StartRaft initializes the node to be part of the raft cluster, the Logger process procedure
// is differente because its will never the first initialize node and never a candidate to leadership
func (lgr *Logger) StartRaft(localID string) error {

	// Setup Raft configuration.
	config := configRaft()
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
	dir := "checkpoints/" + logID
	snapshots, err := raft.NewFileSnapshotStore(dir, 2, os.Stderr)
	if err != nil {
		return fmt.Errorf("file snapshot store: %s", err)
	}

	// Instantiate the Raft systems.
	ra, err := raft.NewRaft(config, (*fsm)(lgr), logStore, stableStore, snapshots, transport)
	if err != nil {
		return fmt.Errorf("new raft: %s", err)
	}
	lgr.raft = ra
	return nil
}

var logID string
var raftAddr string
var joinAddr string

var logfolder *string

func init() {
	flag.StringVar(&logID, "id", "", "Set the logger unique ID")
	flag.StringVar(&raftAddr, "raft", ":12000", "Set RAFT consensus bind address")
	flag.StringVar(&joinAddr, "join", ":13000", "Set join address to an already configured raft node")

	logfolder = flag.String("logfolder", "", "log received commands to a file at specified destination folder using Journey")
}

func main() {

	flag.Parse()
	if logID == "" {
		log.Fatalln("must set a logger ID, run with: ./logger -id 'logID'")
	}
	logger := NewLogger()

	// Start the Raft cluster
	if err := logger.StartRaft(logID); err != nil {
		log.Fatalf("failed to start raft cluster: %s", err.Error())
	}
	if err := sendJoinRequest(); err != nil {
		log.Fatalf("failed to send join request to node at %s: %s", joinAddr, err.Error())
	}

	terminate := make(chan os.Signal, 1)
	signal.Notify(terminate, os.Interrupt)
	<-terminate
	logger.LogFile.Close()
}

func sendJoinRequest() error {
	joinConn, err := net.Dial("tcp", joinAddr)
	if err != nil {
		return err
	}

	_, err = fmt.Fprint(joinConn, logID+"-"+raftAddr+"-"+"false"+"\n")
	if err != nil {
		return err
	}

	err = joinConn.Close()
	if err != nil {
		return err
	}
	return nil
}
