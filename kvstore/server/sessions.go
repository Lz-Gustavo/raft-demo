package main

import (
	"bufio"
	"context"
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
	cancel   context.CancelFunc
}

// NewSession instantiates a new client
func NewSession(connection net.Conn) *Session {

	reader := bufio.NewReader(connection)
	writer := bufio.NewWriter(connection)
	ctx, c := context.WithCancel(context.Background())

	client := &Session{
		incoming: make(chan *Request),
		outgoing: make(chan string),
		reader:   reader,
		writer:   writer,
		conn:     connection,
		cancel:   c,
	}

	client.Listen(ctx)
	return client
}

func (client *Session) Read(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return

		default:
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
}

func (client *Session) Write(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return

		case data := <-client.outgoing:
			client.writer.WriteString(data)
		}
	}
}

// Listen launches Read and Write for every new client connected, async.
// sending/receiving messages following publish/subscriber pattern
func (client *Session) Listen(ctx context.Context) {
	go client.Read(ctx)
	go client.Write(ctx)
}

// Disconnect closes both in and out channels, consequently panicking
// Read and Write goroutines
func (client *Session) Disconnect() {
	client.cancel()
	client.conn.Close()
}
