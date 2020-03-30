package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	staticIPs = false
)

var (
	logID            string
	raftAddr         string
	joinAddr         string
	recovHandlerAddr string
	logfolder        *string
)

func init() {

	if staticIPs {
		parseIPsFromArgsConfig()
	} else {
		err := requestKubeConfig()
		if err != nil {
			log.Fatalln("Failed to retrieve Kubernetes config, err:", err.Error())
		}
	}

	flag.StringVar(&logID, "id", "", "Set the logger unique ID")
	logfolder = flag.String("logfolder", "", "log received commands to a file at specified destination folder")
	flag.Parse()
	if logID == "" {
		log.Fatalln("must set a logger ID, run with: ./logger -id 'logID'")
	}
}

func main() {

	listOfLogIds := strings.Split(logID, ",")
	numDiffIds := countDiffStrInSlice(listOfLogIds)

	listOfRaftAddrs := strings.Split(raftAddr, ",")
	numDiffRaft := countDiffStrInSlice(listOfRaftAddrs)

	listOfJoinAddrs := strings.Split(joinAddr, ",")
	numDiffServices := countDiffStrInSlice(listOfJoinAddrs)

	if numDiffServices != numDiffIds || numDiffIds != numDiffRaft || numDiffRaft != numDiffServices {
		log.Fatalln("must run with the same number of unique IDs, raft and join addrs: ./logger -id 'X,Y' -raft 'A,B' -join 'W,Z'")
	}

	loggerInstances := make([]*Logger, numDiffServices)
	for i := 0; i < numDiffServices; i++ {
		go func(j int) {

			loggerInstances[j] = NewLogger(listOfLogIds[j])
			if err := loggerInstances[j].StartRaft(listOfLogIds[j], listOfRaftAddrs[j]); err != nil {
				log.Fatalf("failed to start raft cluster: %s", err.Error())
			}
			if err := sendJoinRequest(listOfLogIds[j], listOfRaftAddrs[j], listOfJoinAddrs[j]); err != nil {
				log.Fatalf("failed to send join request to node at %s: %s", listOfJoinAddrs[j], err.Error())
			}
		}(i)
	}

	terminate := make(chan os.Signal, 1)
	signal.Notify(terminate, os.Interrupt)
	<-terminate

	for _, l := range loggerInstances {
		l.cancel()
		l.LogFile.Close()
	}
}

func sendJoinRequest(logID, raftAddr, joinAddr string) error {
	joinConn, err := net.Dial("tcp", joinAddr)
	if err != nil {
		return err
	}
	_, err = fmt.Fprint(joinConn, logID+"-"+raftAddr+"-"+"false"+"\n")
	if err != nil {
		return err
	}
	err = joinConn.Close()
	if err != nil {
		return err
	}
	return nil
}

func countDiffStrInSlice(elements []string) int {

	foundMarker := make(map[string]bool, len(elements))
	numDiff := 0

	for _, str := range elements {
		if !foundMarker[str] {
			foundMarker[str] = true
			numDiff++
		}
	}
	return numDiff
}

func parseIPsFromArgsConfig() {
	flag.StringVar(&raftAddr, "raft", ":12000", "Set RAFT consensus bind address")
	flag.StringVar(&joinAddr, "join", ":13000", "Set join address to an already configured raft node")
	flag.StringVar(&recovHandlerAddr, "hrecov", "", "Set port id to receive state transfer requests from the application log")
}

// requestKubeConfig with a different implementation than the other applications, submmiting
// join requests to every identified pod during initilization.
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

	for _, pod := range pods.Items {
		fmt.Println("IP:", pod.Status.PodIP)
	}
	return nil
}
