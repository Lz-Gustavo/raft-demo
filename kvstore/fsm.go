package main

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"strings"

	"github.com/Lz-Gustavo/beelog/pb"

	"github.com/golang/protobuf/proto"
	"github.com/hashicorp/raft"
)

type fsm Store

// Apply applies a Raft log entry to the key-value store.
func (f *fsm) Apply(l *raft.Log) interface{} {

	cmd := &pb.Command{}
	err := proto.Unmarshal(l.Data, cmd)
	if err != nil {
		return err
	}

	err = f.LogCommand(l.Index, cmd, f.Logging)
	if err != nil {
		panic(fmt.Sprintf("couldnt log command: %v", *cmd))
	}

	switch cmd.Op {
	case pb.Command_SET:
		return strings.Join([]string{cmd.Ip, f.applySet(cmd.Key, cmd.Value)}, "-")
	case pb.Command_GET:
		return strings.Join([]string{cmd.Ip, f.applyGet(cmd.Key)}, "-")
	// case pb.Command_DELETE:
	// 	return strings.Join([]string{cmd.Ip, f.applyDelete(cmd.Key)}, "-")
	default:
		panic(fmt.Sprintf("unrecognized command op: %v", cmd))
	}
}

// Snapshot returns a snapshot of the key-value store.
func (f *fsm) Snapshot() (raft.FSMSnapshot, error) {
	// Clone the map.
	o := make(map[string][]byte)
	for k, v := range f.m {
		o[k] = v
	}
	return &fsmSnapshot{store: o}, nil
}

// Restore stores the key-value store to a previous state.
func (f *fsm) Restore(rc io.ReadCloser) error {
	o := make(map[string][]byte)
	if err := json.NewDecoder(rc).Decode(&o); err != nil {
		return err
	}

	// Set the state from the snapshot, no lock required according to
	// Hashicorp docs.
	f.m = o
	return nil
}

// NOTE: There s no need for mutex acquisition since every new command is garantee to be
// executed in a sequential manner, preserving the replicas coordination.
func (f *fsm) applySet(key, value string) string {
	if !f.compress {
		f.m[key] = []byte(value)
		return ""
	}

	f.gzipBuffer.Reset()
	wtr := gzip.NewWriter(&f.gzipBuffer)
	wtr.Write([]byte(value))

	if err := wtr.Flush(); err != nil {
		panic(err)
	}
	if err := wtr.Close(); err != nil {
		panic(err)
	}

	f.m[key] = f.gzipBuffer.Bytes()
	return ""
}

func (f *fsm) applyDelete(key string) string {
	delete(f.m, key)
	return ""
}

// NOTE: pre-initalization of gzipReader on fsm store is not viable option, because necessary
// Close() calls from write method will imediately dealloc the f.Reader attribute. This closure
// is necessary to prevent io.ErrUnexpectedEOF
func (f *fsm) applyGet(key string) string {
	value, ok := f.m[key]
	if !ok {
		return ""
	}
	if !f.compress {
		return string(f.m[key])
	}

	f.gzipBuffer.Reset()
	rd := bytes.NewReader(value)
	rdGzip, _ := gzip.NewReader(rd)
	bytes, _ := ioutil.ReadAll(rdGzip)
	rdGzip.Close()
	return string(bytes)
}

type fsmSnapshot struct {
	store map[string][]byte
}

func (f *fsmSnapshot) Persist(sink raft.SnapshotSink) error {
	err := func() error {
		// Encode data.
		b, err := json.Marshal(f.store)
		if err != nil {
			return err
		}

		// Write data to sink.
		if _, err := sink.Write(b); err != nil {
			return err
		}
		return sink.Close()
	}()
	if err != nil {
		sink.Cancel()
	}

	return err
}

func (f *fsmSnapshot) Release() {}

// LogCommand logs the received command on the choosen index following the configured
// log strategy.
func (f *fsm) LogCommand(ind uint64, cmd *pb.Command, st LogStrategy) error {
	cmd.Id = ind
	switch st {
	case NotLog:
		return nil

	case DiskTrad:
		rawCmd, err := proto.Marshal(cmd)
		if err != nil {
			return err
		}

		binary.Write(f.LogFile, binary.BigEndian, int32(len(rawCmd)))
		f.LogFile.Write(rawCmd)
		break

	case InmemTrad:
		if f.inMemLog == nil {
			f.inMemLog = &[]pb.Command{}
		}
		*f.inMemLog = append(*f.inMemLog, *cmd)
		break

	case DiskBeelog: // same log procedure
	case InmemBeelog:
		err := f.avl.Log(ind, *cmd)
		if err != nil {
			return err
		}
		break

	default:
		return fmt.Errorf("unsupported log strategy")
	}
	return nil
}
