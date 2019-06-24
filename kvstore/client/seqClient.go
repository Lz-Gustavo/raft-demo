package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/Lz-Gustavo/journey/pb"
	"github.com/golang/protobuf/proto"
)

// Info stores the server configuration
type Info struct {
	Rep    int
	SvrIps []string

	Svrs   []net.Conn
	reader []*bufio.Reader

	Localip  string
	Udpport  int
	receiver *net.UDPConn
}

// New instatiates a new sequential client config struct from toml file.
// Follows a seq behavioral, sending new msgs just after receiving their
// reponse. It does not implement channels publish-subscriber pattern
// because it results in a burst of requisitions to the servers.
func New(config string) (*Info, error) {

	info := &Info{}
	_, err := toml.DecodeFile(config, info)
	if err != nil {
		return nil, err
	}
	return info, nil
}

// Connect creates a tcp connection to every replica on the cluster
func (client *Info) Connect() error {

	client.Svrs = make([]net.Conn, client.Rep)
	client.reader = make([]*bufio.Reader, client.Rep)
	var err error

	for i, v := range client.SvrIps {
		client.Svrs[i], err = net.Dial("tcp", v)
		if err != nil {
			return err
		}
		client.reader[i] = bufio.NewReader(client.Svrs[i])
	}
	return nil
}

// Disconnect closes every open socket connection with the fsm cluster
func (client *Info) Disconnect() {
	for _, v := range client.Svrs {
		v.Close()
	}
}

// StartUDP initializes UDP listener, used to receive servers repplies
func (client *Info) StartUDP() error {

	addr := net.UDPAddr{
		IP:   net.ParseIP(client.Localip),
		Port: client.Udpport,
		Zone: "",
	}
	conn, err := net.ListenUDP("udp", &addr)
	if err != nil {
		return err
	}
	client.receiver = conn
	return nil
}

// Broadcast a message to the cluster
func (client *Info) Broadcast(message string) error {
	for _, v := range client.Svrs {
		_, err := fmt.Fprint(v, strconv.Itoa(client.Udpport)+"-"+message)
		if err != nil {
			return err
		}
	}
	return nil
}

// BroadcastProtobuf sends a serialized command to the cluster
func (client *Info) BroadcastProtobuf(message *pb.Command, clientUDPPort string) error {

	message.Ip = clientUDPPort
	serializedMessage, err := proto.Marshal(message)
	if err != nil {
		return err
	}
	serializedMessage = append(serializedMessage, []byte("\n")...)

	for _, v := range client.Svrs {
		_, err := v.Write(serializedMessage)
		if err != nil {
			return err
		}
	}
	return nil
}

// ReadTCP consumes any data from reader socket and returns it
func (client *Info) ReadTCP(readerID int) string {
	line, err := client.reader[readerID].ReadString('\n')
	if (err == nil) && (len(line) > 1) {
		return line
	}
	return ""
}

// ReadTCPParallel launches go routines to read from every socket connected to
// the cluster, and returns the first response.
//
// TODO: currently it blocks during ReadString() invocation, not releasing
// the go routine until it receives the delim from the socket. Thats not a
// desired behavior, since it may be expecting repplies from a faulty replica
func (client *Info) ReadTCPParallel() string {

	chn := make(chan string, client.Rep)
	for i := range client.reader {

		go func(j int, rp chan<- string) {

			line, err := client.reader[j].ReadString('\n')
			if err == nil {
				rp <- line
			}
			return
		}(i, chn)
	}
	return <-chn
}

// ReadUDP returns any received message from UDP listener for servers reppply
func (client *Info) ReadUDP() (string, error) {

	data := make([]byte, 128)
	_, _, err := client.receiver.ReadFromUDP(data)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// Shutdown realeases every resource and finishes goroutines launched by the
// client programm
func (client *Info) Shutdown() {
	client.Disconnect()
}

var configFilename *string

func init() {
	configFilename = flag.String("config", "", "Filepath to toml file")
}

func main() {

	flag.Parse()
	if *configFilename == "" {
		log.Fatalln("must set a config filepath: ./client -config '../config.toml'")
	}

	cluster, err := New(*configFilename)
	if err != nil {
		log.Fatalf("failed to find config: %s", err.Error())
	}

	fmt.Println("rep:", cluster.Rep)
	fmt.Println("svrIps:", cluster.SvrIps)
	fmt.Println("udpaddr:", cluster.Udpport)

	err = cluster.Connect()
	if err != nil {
		log.Fatalf("failed to connect to cluster: %s", err.Error())
	}

	err = cluster.StartUDP()
	if err != nil {
		log.Fatalf("failed to initialize UDP socket: %s", err.Error())
	}

	reader := bufio.NewReader(os.Stdin)
	for {

		text, err := reader.ReadString('\n')
		if err != nil {
			log.Printf("input reader failed: %s", err.Error())
			continue
		}
		if strings.HasPrefix(text, "exit") {
			cluster.Shutdown()
			break
		}
		err = cluster.Broadcast(text + "\n")
		if err != nil {
			log.Printf("broadcast failed: %s", err.Error())
			continue
		}

		// TODO: These serialized calls to Read() are inneficient, must find a way
		// to execute an async read to every socket with a better performance
		// for i := range cluster.reader {
		// 	repply := cluster.ReadTCP(i)
		// 	if repply != "" {
		// 		fmt.Printf("Received message: %s", repply)
		// 		break
		// 	}
		// }
		//repply := cluster.ReadTCPParallel()

		repply, _ := cluster.ReadUDP()
		fmt.Printf("Received message: %s", repply)
	}
}
