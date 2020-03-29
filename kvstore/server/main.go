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

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var svrID string
var svrPort string
var raftAddr string
var joinAddr string
var joinHandlerAddr string
var recovHandlerAddr string

var cpuprofile, memprofile, logfolder *string

const (
	staticConfig = true
)

func init() {

	if staticConfig {
		parseArgsConfig()
		if svrID == "" {
			log.Fatalln("Must set a server ID, run with: ./server -id 'svrID'")
		}

	} else {
		err := requestKubeConfig()
		if err != nil {
			log.Fatalln("Failed to retrieve Kubernetes config, err:", err.Error())
		}
	}
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

func parseArgsConfig() {
	flag.StringVar(&svrID, "id", "", "Set server unique ID")
	flag.StringVar(&svrPort, "port", ":11000", "Set the server bind address")
	flag.StringVar(&raftAddr, "raft", ":12000", "Set RAFT consensus bind address")
	flag.StringVar(&joinAddr, "join", "", "Set join address, if any")
	flag.StringVar(&joinHandlerAddr, "hjoin", "", "Set port id to receive join requests on the raft cluster")
	flag.StringVar(&recovHandlerAddr, "hrecov", "", "Set port id to receive state transfer requests from the application log")

	cpuprofile = flag.String("cpuprofile", "", "write cpu profile to a file")
	memprofile = flag.String("memprofile", "", "write memory profile to a file")
	logfolder = flag.String("logfolder", "", "log received commands to a file at specified destination folder")
	flag.Parse()
}

func requestKubeConfig() error {

	// creates the in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		return err
	}

	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}

	// get pods in all the namespaces by omitting namespace
	// Or specify namespace to get pods in particular namespace
	pods, err := clientset.CoreV1().Pods("").List(metav1.ListOptions{})
	if err != nil {
		return err
	}
	fmt.Printf("There are %d pods in the cluster\n", len(pods.Items))

	// Examples for error handling:
	// - Use helper functions e.g. errors.IsNotFound()
	// - And/or cast to StatusError and use its properties like e.g. ErrStatus.Message
	_, err = clientset.CoreV1().Pods("default").Get("example-xxxxx", metav1.GetOptions{})
	if errors.IsNotFound(err) {
		fmt.Printf("Pod example-xxxxx not found in default namespace\n")
	} else if statusError, isStatus := err.(*errors.StatusError); isStatus {
		fmt.Printf("Error getting pod %v\n", statusError.ErrStatus.Message)
	} else if err != nil {
		return err
	} else {
		fmt.Printf("Found example-xxxxx pod in default namespace\n")
	}
	return nil
}
