package main

import (
	"bufio"
	"fmt"
	"net"
)

// Session struct which represents each ...
type Session struct {
	incoming chan string
	outgoing chan string
	reader   *bufio.Reader
	writer   *bufio.Writer
}

// NewSession instantiates a new client
func NewSession(connection net.Conn) *Session {
	writer := bufio.NewWriter(connection)
	reader := bufio.NewReader(connection)

	client := &Session{
		incoming: make(chan string),
		outgoing: make(chan string),
		reader:   reader,
		writer:   writer,
	}

	client.Listen()
	return client
}

func (client *Session) Read() {
	for {
		line, err := client.reader.ReadString('\n')
		if (err == nil) && (len(line) > 1) {
			fmt.Println("Received message: ", line)
			client.incoming <- line
		}
	}
}

func (client *Session) Write() {
	for data := range client.outgoing {
		client.writer.WriteString(data)
		client.writer.Flush()
	}
}

// Listen ...
func (client *Session) Listen() {
	go client.Read()
	go client.Write()
}

// Disconnect ...
func (client *Session) Disconnect() {

	close(client.incoming)
	close(client.outgoing)
}
