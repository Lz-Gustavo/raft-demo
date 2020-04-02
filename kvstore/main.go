package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"runtime"
	"runtime/pprof"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	// Defines wheter the application should interpret IPs provided on
	// cmdli args or use its current env POD_IP and ask the other to
	// Kubernetes sdk
	staticIPs = false
)

var (
	svrID            string
	svrPort          string
	raftAddr         string
	joinAddr         string
	joinHandlerAddr  string
	recovHandlerAddr string
	cpuprofile       *string
	memprofile       *string
	logfolder        *string

	envPodIP        string
	envPodName      string
	envPodNamespace string
)

func init() {

	if staticIPs {
		parseIPsFromArgsConfig()
	} else {

		loadEnvVariables()
		svrID = "node" + strings.Split(envPodIP, ".")[3]
		svrPort = ":11000"
		raftAddr = envPodIP + ":12000"

		err := requestKubeConfig()
		if err != nil {
			log.Fatalln("Failed to retrieve Kubernetes config, err:", err.Error())
		}
	}

	cpuprofile = flag.String("cpuprofile", "", "write cpu profile to a file")
	memprofile = flag.String("memprofile", "", "write memory profile to a file")
	logfolder = flag.String("logfolder", "", "log received commands to a file at specified destination folder")
	flag.Parse()

	if svrID == "" {
		log.Fatalln("Must set a server ID, run with: ./server -id 'svrID'")
	}

	fmt.Println("ID:", svrID)
	fmt.Println("app:", svrPort)
	fmt.Println("raft:", raftAddr)
	fmt.Println("join:", joinAddr)
	fmt.Println("hjoin:", joinHandlerAddr)
}

func main() {

	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			log.Fatal("could not create CPU profile: ", err)
		}
		defer f.Close()
		if err := pprof.StartCPUProfile(f); err != nil {
			log.Fatal("could not start CPU profile: ", err)
		}
		defer pprof.StopCPUProfile()
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Initialize the Key-value store
	kvs := New(ctx, true)
	listener, err := net.Listen("tcp", svrPort)
	if err != nil {
		log.Fatalf("failed to start connection: %s", err.Error())
	}

	// Start the Raft cluster
	if err := kvs.StartRaft(joinAddr == "", svrID, raftAddr); err != nil {
		log.Fatalf("failed to start raft cluster: %s", err.Error())
	}

	// Initialize the server
	server := NewServer(ctx, kvs)

	// Send a join request, if any
	if joinAddr != "" {
		if err = sendJoinRequest(); err != nil {
			log.Fatalf("failed to send join request to node at %s: %s", joinAddr, err.Error())
		}
	}

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				log.Fatalf("accept failed: %s", err.Error())
			}

			server.joins <- conn
			server.kvstore.logger.Info("New client connected!")
		}
	}()

	terminate := make(chan os.Signal, 1)
	signal.Notify(terminate, os.Interrupt)
	<-terminate

	if *memprofile != "" {
		f, err := os.Create(*memprofile)
		if err != nil {
			log.Fatal("could not create memory profile: ", err)
		}
		defer f.Close()
		runtime.GC() // get up-to-date statistics
		if err := pprof.WriteHeapProfile(f); err != nil {
			log.Fatal("could not write memory profile: ", err)
		}
	}

	cancel()
	server.Exit()
}

func sendJoinRequest() error {

	joinConn, err := net.Dial("tcp", joinAddr)
	if err != nil {
		return fmt.Errorf("failed to connect to leader node at %s: %s", joinAddr, err.Error())
	}

	_, err = fmt.Fprint(joinConn, svrID+"-"+raftAddr+"-"+"true"+"\n")
	if err != nil {
		return fmt.Errorf("failed to send join request to node at %s: %s", joinAddr, err.Error())
	}

	if err = joinConn.Close(); err != nil {
		return err
	}
	return nil
}

func parseIPsFromArgsConfig() {
	flag.StringVar(&svrID, "id", "", "Set server unique ID")
	flag.StringVar(&svrPort, "port", ":11000", "Set the server bind address")
	flag.StringVar(&raftAddr, "raft", ":12000", "Set RAFT consensus bind address")
	flag.StringVar(&joinAddr, "join", "", "Set join address, if any")
	flag.StringVar(&joinHandlerAddr, "hjoin", "", "Set port id to receive join requests on the raft cluster")
	flag.StringVar(&recovHandlerAddr, "hrecov", "", "Set port id to receive state transfer requests from the application log")
}

func requestKubeConfig() error {

	if isLeader() {

		// Must only set port 13000 to listen raft join request invoked by
		// loggers and followers
		joinHandlerAddr = ":13000"

	} else {

		config, err := rest.InClusterConfig()
		if err != nil {
			return err
		}

		clientset, err := kubernetes.NewForConfig(config)
		if err != nil {
			return err
		}

		// get only pods in the current namespace
		pods, err := clientset.CoreV1().Pods(envPodNamespace).List(metav1.ListOptions{})
		if err != nil {
			return err
		}

		//wait for the leader pod IP attribution...
		time.Sleep(time.Duration(3 * time.Second))

		for _, pod := range pods.Items {
			// fmt.Println(
			// 	"Pod #:", i,
			// 	"IP:", pod.Status.PodIP,
			// 	"\n====Status:", pod.Status, "\n\n",
			// )

			// The leader pod status...
			if strings.Contains(pod.Status.ContainerStatuses[0].Name, "leader") {

				if pod.Status.PodIP == "" {
					log.Fatalln("forcing a container restart...")
				}

				// Later send a join request to the leaders IP.
				joinAddr = pod.Status.PodIP + ":13000"
			}
		}
	}
	return nil
}

func loadEnvVariables() {

	var ok bool
	envPodIP, ok = os.LookupEnv("MY_POD_IP")
	if !ok {
		log.Fatalln("could not load environment variable MY_POD_IP")
	}
	fmt.Println("retrieved MY_POD_IP:", envPodIP)

	envPodName, ok = os.LookupEnv("MY_POD_NAME")
	if !ok {
		log.Fatalln("could not load environment variable MY_POD_NAME")
	}
	fmt.Println("retrieved MY_POD_NAME:", envPodName)

	envPodNamespace, ok = os.LookupEnv("MY_POD_NAMESPACE")
	if !ok {
		log.Fatalln("could not load environment variable MY_POD_NAMESPACE")
	}
	fmt.Println("retrieved MY_POD_NAMESPACE:", envPodNamespace)
}

func isLeader() bool {
	return strings.Contains(envPodName, "leader")
}