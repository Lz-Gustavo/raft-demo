package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
)

// Info stores the server configuration
type Info struct {
	Rep    int
	SvrIps []string
	LogRep int
	LogIps []string

	Svrs []net.Conn
	Logs []net.Conn

	reader   []*bufio.Reader
	incoming chan string
}

// New instatiates a new client config struct from toml file
func New() (*Info, error) {

	info := &Info{}

	_, err := toml.DecodeFile("../config.toml", info)
	if err != nil {
		return nil, err
	}

	info.incoming = make(chan string)
	return info, nil
}

// Connect creates a tcp connection to every replica on the fsm
func (client *Info) Connect() error {

	client.Svrs = make([]net.Conn, client.Rep)
	client.reader = make([]*bufio.Reader, client.Rep)
	client.Logs = make([]net.Conn, client.LogRep)
	var err error

	for i, v := range client.SvrIps {
		client.Svrs[i], err = net.Dial("tcp", v)
		if err != nil {
			return err
		}

		client.reader[i] = bufio.NewReader(client.Svrs[i])
		go client.Read(i)
	}

	for i, v := range client.LogIps {
		client.Logs[i], err = net.Dial("tcp", v)
		if err != nil {
			return err
		}
	}

	go client.Consume()
	return nil
}

// Disconnect closes every open socket connection with the fsm
// cluster
func (client *Info) Disconnect() {

	for _, v := range client.Svrs {
		v.Close()
	}
	for _, v := range client.Logs {
		v.Close()
	}
}

// Broadcast a message to the cluster
func (client *Info) Broadcast(message string) error {
	for _, v := range client.Svrs {
		_, err := fmt.Fprint(v, message)
		if err != nil {
			return err
		}
	}
	return nil
}

// Read consumes any data from reader socket and stores it into the
// incoming channel
func (client *Info) Read(readerID int) {
	for {
		line, err := client.reader[readerID].ReadString('\n')
		if (err == nil) && (len(line) > 1) {
			client.incoming <- strconv.Itoa(readerID) + "-" + line
		}
	}
}

// Consume reads any data from incoming channel and outputs it
func (client *Info) Consume() {
	for {
		v, ok := <-client.incoming
		if !ok {
			break
		}
		fmt.Println("Received message:", v)
	}
}

// Shutdown realeases every resource and finishes goroutines launched
// by the client programm
func (client *Info) Shutdown() {

	client.Disconnect()
	close(client.incoming)
}

func main() {

	cluster, err := New()
	if err != nil {
		log.Fatalf("failed to find config: %s", err.Error())
	}

	fmt.Println("rep:", cluster.Rep)
	fmt.Println("svrIps:", cluster.SvrIps)
	fmt.Println("logRep:", cluster.LogRep)
	fmt.Println("logIps:", cluster.LogIps)

	err = cluster.Connect()
	if err != nil {
		log.Fatalf("failed to connect to cluster: %s", err.Error())
	}

	reader := bufio.NewReader(os.Stdin)
	for {

		text, err := reader.ReadString('\n')
		if err != nil {
			log.Printf("input reader failed: %s", err.Error())
			continue
		}
		if strings.Contains(text, "exit") {
			cluster.Shutdown()
			break
		}

		err = cluster.Broadcast(text + "\n")
		if err != nil {
			log.Printf("broadcast failed: %s", err.Error())
			continue
		}
	}
}
