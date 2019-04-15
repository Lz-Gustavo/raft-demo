package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"os"

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
}

// New instatiates a new client config struct from toml file
func New() (*Info, error) {

	info := &Info{}

	_, err := toml.DecodeFile("../config.toml", info)
	if err != nil {
		return nil, err
	}
	return info, nil
}

// Connect creates a tcp connection to every replica on the fsm
func (client *Info) Connect() error {

	client.Svrs = make([]net.Conn, client.Rep)
	client.Logs = make([]net.Conn, client.LogRep)
	var err error

	for i, v := range client.SvrIps {
		client.Svrs[i], err = net.Dial("tcp", v)
		if err != nil {
			return err
		}
	}

	for i, v := range client.LogIps {
		client.Logs[i], err = net.Dial("tcp", v)
		if err != nil {
			return err
		}
	}
	return nil
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

		err = cluster.Broadcast(text + "\n")
		if err != nil {
			log.Printf("broadcast failed: %s", err.Error())
			continue
		}
	}

}
