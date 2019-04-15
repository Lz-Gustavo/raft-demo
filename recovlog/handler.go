package recovlog

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"sync"
	"sync/atomic"

	"github.com/BurntSushi/toml"
)

type writeMethod int

// Indexes of each different handler implementation
const (
	Naive writeMethod = iota
	Syncr
	Batch
)

type handler struct {
	Local *os.File
	Type  writeMethod

	Rep   bool
	IPs   []string
	Group []net.Conn

	Sync  bool
	Mutex sync.Mutex

	BatchSize int32
	Buffer    *bufio.Writer
	//CountMutex sync.Mutex
	Count int32
}

func (h *handler) connect() error {

	h.Group = make([]net.Conn, len(h.IPs))
	var err error

	for i, value := range h.IPs {

		h.Group[i], err = net.Dial("tcp", value)
		if err != nil {
			return err
		}
	}
	return nil
}

func (h *handler) disconnect() {
	for i := range h.Group {
		h.Group[i].Close()
	}
}

func (h *handler) New(configpath string) error {

	_, err := toml.DecodeFile(configpath, h)
	if err != nil {
		return err
	}

	if h.Rep {
		err := h.connect()
		if err != nil {
			return err
		}
	}

	// Not deleting if exists while testing in localhost
	if _, exists := os.Stat("log-file.txt"); exists == nil {
		//os.Remove("log-file.txt")
		h.Local, err = os.OpenFile("log-file.txt", os.O_APPEND|os.O_WRONLY, 0755)
	} else if os.IsNotExist(exists) {
		h.Local, err = os.Create("log-file.txt")
	}

	if h.BatchSize > 0 {
		h.Buffer = bufio.NewWriter(h.Local)
		h.Buffer = bufio.NewWriterSize(h.Buffer, int(h.BatchSize))
		h.Count = 0
		h.Type = Batch
	} else if h.Sync {
		h.Type = Syncr
	} else {
		h.Type = Naive
	}

	return err
}

func (h *handler) Delete() {

	defer h.Local.Close()
	defer atomic.StoreInt32(&h.Count, 0)

	if h.Rep {
		h.disconnect()
	}
	if h.Buffer != nil {
		h.Mutex.Lock()
		defer h.Mutex.Unlock()
		h.Buffer.Flush()
	}
}

func (h *handler) Write(c Command) {

	switch h.Type {
	case Batch:
		h.BatchSave(c)
		break

	case Syncr:
		h.SyncSave(c)
		break

	case Naive:
		h.Save(c)
		break
	}
}

func (h *handler) Save(c Command) {
	fmt.Fprintf(h.Local, "%d -- %d --    %s    | %s\n", c.id, c.cmd, c.comment, c.ts)
}

// SyncSave utilizes an internal mutex to guarantee syncronized Save() calls, necessary
// in a concurrent scenario.
func (h *handler) SyncSave(c Command) {

	defer h.Mutex.Unlock()
	h.Mutex.Lock()
	fmt.Fprintf(h.Local, "%d -- %d --    %s    | %s\n", c.id, c.cmd, c.comment, c.ts)
}

// BatchSave uses a bufio to avoid repetitive and costly Fprint calls on disk, and does a
// batch update with w.Flush() after 'h.BatchSize' saves
func (h *handler) BatchSave(c Command) {

	h.Mutex.Lock()
	defer h.Mutex.Unlock()
	fmt.Fprintf(h.Buffer, "%d -- %d --    %s    | %s\n", c.id, c.cmd, c.comment, c.ts)
	h.Count++

	if h.Count >= h.BatchSize {
		h.Buffer.Flush()
		h.Count = 0
	}
	// atomic.CompareAndSwapInt32(&h.Count, int32(h.Batch-1), 0)
}

// TODO: Implement a fan-out pattern to distributed the same write operation to all connected
// nodes.
func (h *handler) Multicast(c Command) {
}

func (h *handler) Debug() {
	fmt.Println("Config file content:")
	fmt.Println(h.Rep)
	fmt.Println(h.IPs)
	fmt.Println(h.Sync)
	fmt.Println(h.BatchSize)
}
