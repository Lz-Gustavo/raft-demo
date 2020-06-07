package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
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
	staticIPs = true
)

var (
	numApps          int
	logIDs           []string
	raftAddrs        []string
	joinAddrs        []string
	recovHandlerAddr string
	logfolder        *string

	envPodIP        string
	envPodName      string
	envPodNamespace string
)

func init() {

	logfolder = flag.String("logfolder", "/tmp/", "log received commands to a file at specified destination folder")
	if staticIPs {
		err := parseIPsFromArgsConfig()
		if err != nil {
			log.Fatalln("could not parse cmdli args, err:", err.Error())
		}

	} else {

		err := loadEnvVariables()
		if err != nil {
			log.Fatalln(err.Error())
		}

		err = requestKubeConfig()
		if err != nil {
			log.Fatalln("failed to retrieve Kubernetes config, err:", err.Error())
		}

		go launchPsutilMonitor()

		// Raft participants must posses an unique identifier within the cluster,
		// this is a non optional way to assure that. Considering that a logger
		// gets a rand 'a', if another logger gets a baserand 'b'
		//
		//   b < (a - numApplications) for all b in {0..RAND_MAX}
		//
		// must always be satisfied to avoid matching intervals for logID values.
		baseRand := rand.Intn(10000)

		basePort := 12000
		for i := range joinAddrs {

			id := "log" + strconv.Itoa(baseRand+i)
			logIDs = append(logIDs, id)

			raft := envPodIP + ":" + strconv.Itoa(basePort+i)
			raftAddrs = append(raftAddrs, raft)
		}
	}
	debugLoggerState()
}

func main() {

	loggerInstances := make([]*Logger, numApps)
	for i := 0; i < numApps; i++ {
		go func(j int) {

			loggerInstances[j] = NewLogger(logIDs[j])
			if err := loggerInstances[j].StartRaft(logIDs[j], raftAddrs[j]); err != nil {
				log.Fatalf("failed to start raft cluster: %s", err.Error())
			}
			if err := sendJoinRequest(logIDs[j], raftAddrs[j], joinAddrs[j]); err != nil {
				log.Fatalf("failed to send join request to node at %s: %s", joinAddrs[j], err.Error())
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

func parseIPsFromArgsConfig() error {

	var logs, raft, joins string
	flag.StringVar(&logs, "id", "", "Set the logger unique ID")
	flag.StringVar(&raft, "raft", ":12000", "Set RAFT consensus bind address")
	flag.StringVar(&joins, "join", ":13000", "Set join address to an already configured raft node")
	flag.StringVar(&recovHandlerAddr, "hrecov", "", "Set port id to receive state transfer requests from the application log")
	flag.Parse()

	if logs == "" {
		return errors.New("must set a logger ID, run with: ./logger -id 'logID'")
	}

	logIDs = strings.Split(logs, ",")
	numDiffIds := countDiffStrInSlice(logIDs)

	raftAddrs = strings.Split(raft, ",")
	numDiffRaft := countDiffStrInSlice(raftAddrs)

	joinAddrs = strings.Split(joins, ",")
	numDiffServices := countDiffStrInSlice(joinAddrs)

	inconsistentQtd := numDiffServices != numDiffIds || numDiffIds != numDiffRaft || numDiffRaft != numDiffServices
	if inconsistentQtd {
		return errors.New("must run with the same number of unique IDs, raft and join addrs: ./logger -id 'X,Y' -raft 'A,B' -join 'W,Z'")
	}
	numApps = numDiffIds
	return nil
}

// requestKubeConfig with a different implementation than the other applications, submmiting
// join requests to every identified pod during initilization.
func requestKubeConfig() error {

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

		// The leader pod status...
		if strings.Contains(pod.Status.ContainerStatuses[0].Name, "leader") {

			if pod.Status.PodIP == "" {
				log.Fatalln("forcing a container restart...")
			}

			// Later send a join request to the leaders IP.
			joinAddrs = append(joinAddrs, pod.Status.PodIP+":13000")
		}
	}

	numApps = len(joinAddrs)
	if numApps == 0 {
		log.Fatalln("could not retrieve any leader address, restarting...")
	}
	return nil
}

func loadEnvVariables() error {

	var ok bool
	envPodIP, ok = os.LookupEnv("MY_POD_IP")
	if !ok {
		return errors.New("could not load environment variable MY_POD_IP")
	}
	fmt.Println("retrieved MY_POD_IP:", envPodIP)

	envPodName, ok = os.LookupEnv("MY_POD_NAME")
	if !ok {
		return errors.New("could not load environment variable MY_POD_NAME")
	}
	fmt.Println("retrieved MY_POD_NAME:", envPodName)

	envPodNamespace, ok = os.LookupEnv("MY_POD_NAMESPACE")
	if !ok {
		return errors.New("could not load environment variable MY_POD_NAMESPACE")
	}
	fmt.Println("retrieved MY_POD_NAMESPACE:", envPodNamespace)
	return nil
}

func debugLoggerState() {
	for i := 0; i < numApps; i++ {
		fmt.Println(
			"==========",
			"\nApplication #:", i,
			"\nloggerID:", logIDs[i],
			"\nraft:", raftAddrs[i],
			"\nappIP:", joinAddrs[i],
			"\n==========",
		)
	}
}

func launchPsutilMonitor() {
	cmd := exec.Command("python3", "monit_sys.py", "logger")
	err := cmd.Run()
	if err != nil {
		fmt.Print("could not start monitor:", err.Error())
		return
	}
}
