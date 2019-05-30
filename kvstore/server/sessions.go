package main

import (
	"bufio"
	"io"
	"net"
	"strings"
)

// Session struct which represents each active client session connected
// on the cluster
type Session struct {
	incoming chan string
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
		incoming: make(chan string),
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
		line, err := client.reader.ReadString('\n')
		if err == nil && len(line) > 1 {
			ip := client.conn.RemoteAddr().String()
			ipContent := strings.Split(ip, ":")
			concat := []string{ipContent[0], line}
			client.incoming <- strings.Join(concat, ":")
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
