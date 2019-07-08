package main

import (
	"bufio"
	"io"
	"net"
	"strings"
)

// Request struct represents received requests to the KVstore service.
type Request struct {
	Command []byte
	Ip      string
}

// Session struct which represents each active client session connected
// on the cluster
type Session struct {
	incoming chan *Request
	outgoing chan string
	reader   *bufio.Reader
	writer   *bufio.Writer
	conn     net.Conn
}

// NewSession instantiates a new client
func NewSession(connection net.Conn) *Session {
	reader := bufio.NewReader(connection)
	writer := bufio.NewWriter(connection)

	client := &Session{
		incoming: make(chan *Request),
		outgoing: make(chan string),
		reader:   reader,
		writer:   writer,
		conn:     connection,
	}

	client.Listen()
	return client
}

func (client *Session) Read() {
	for {
		line, err := client.reader.ReadBytes('\n')
		if err == nil && len(line) > 1 {
			ip := client.conn.RemoteAddr().String()
			ipContent := strings.Split(ip, ":")
			newRequest := &Request{line, ipContent[0]}
			client.incoming <- newRequest

		} else if err == io.EOF {
			return
		}
	}
}

func (client *Session) Write() {
	for data := range client.outgoing {
		client.writer.WriteString(data)
		client.writer.Flush()
	}
}

// Listen launches Read and Write for every new client connected, async.
// sending/receiving messages following publish/subscriber pattern
func (client *Session) Listen() {
	go client.Read()
	go client.Write()
}

// Disconnect closes both in and out channels, consequently panicking
// Read and Write goroutines
func (client *Session) Disconnect() {
	close(client.incoming)
	close(client.outgoing)
	client.conn.Close()
}
