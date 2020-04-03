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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var (
	envPodIP        string
	envPodName      string
	envPodNamespace string
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

	ThinkingTimeMsec int
}

// New instatiates a new sequential client config struct from toml file.
// Follows a seq behavioral, sending new msgs just after receiving their
// reponse. It does not implement channels publish-subscriber pattern
// because it results in a burst of requisitions to the servers.
func New(config string) (*Info, error) {

	info := &Info{}

	kubeIps := checkKubernetesEnv()
	if kubeIps != nil {

		fmt.Println("Initializing by kubernetes config...")
		info.Rep = len(kubeIps)
		info.SvrIps = kubeIps
		info.Udpport = 15000
		info.ThinkingTimeMsec = 10

		// will be later overwritten by POD_IP...
		info.Localip = "127.0.0.1"

	} else {

		fmt.Println("Initializing by toml config file...")
		_, err := toml.DecodeFile(config, info)
		if err != nil {
			return nil, err
		}
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

	// TODO: used during Kubernetes deploy, fix it later...
	var udpIP string

	envPodIP, ok := os.LookupEnv("MY_POD_IP")
	if ok {
		udpIP = envPodIP
		fmt.Println("retrieved MY_POD_IP:", envPodIP)
	} else {
		envPodIP = client.Localip
	}

	addr := net.UDPAddr{
		IP:   net.ParseIP(udpIP),
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
	configFilename = flag.String("config", "client-config.toml", "Filepath to toml file")
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

func checkKubernetesEnv() []string {

	var ok bool
	envPodName, ok = os.LookupEnv("MY_POD_NAME")

	// If MY_POD_NAME is set, check the index name suffix
	if ok {
		fmt.Println("retrieved MY_POD_NAME:", envPodName)

		nameTags := strings.Split(envPodName, "-")
		var ind int
		var err error

		// e.g. loadgen-app-1-hashcode
		if len(nameTags) >= 3 {
			ind, err = strconv.Atoi(nameTags[2])
			if err != nil {
				fmt.Println("could not parse env index")
				ind = -1
			}
		} else {
			ind = -1
		}
		ips, err := getAppsFromKube(ind)
		if err != nil {
			log.Fatalln("could not init kubernetes config, err:", err.Error())
		}
		return ips
	}
	return nil
}

func getAppsFromKube(podIndex int) ([]string, error) {

	var ok bool
	envPodNamespace, ok = os.LookupEnv("MY_POD_NAMESPACE")
	if !ok {
		envPodNamespace = "default"
	}
	fmt.Println("retrieved MY_POD_NAMESPACE:", envPodNamespace)

	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	// get only pods in the current namespace
	pods, err := clientset.CoreV1().Pods(envPodNamespace).List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	podFilter := ""

	// Search for matching indexes
	if podIndex > -1 {
		podFilter = "-" + strconv.Itoa(podIndex)
		fmt.Println("Now searching for pods with", podFilter, "suffix pattern")
	}

	podIps := make([]string, 0)
	for _, pod := range pods.Items {

		if strings.Contains(pod.Status.ContainerStatuses[0].Name, podFilter) {

			if pod.Status.PodIP == "" {
				log.Fatalln("forcing a container restart...")
			}

			ip := pod.Status.PodIP + ":11000"
			podIps = append(podIps, ip)
		}
	}
	return podIps, nil
}
