package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Lz-Gustavo/journey/pb"
)

var (
	dataChoice int
	oneTweet   = strings.Repeat("@", 128)  // dataChoice == 0
	oneKB      = strings.Repeat("@", 1024) // dataChoice == 1
	fourKB     = strings.Repeat("@", 4096) // dataChoice == 2
)

// Value to be store on the hashmap.
var storeValue string

const (
	// One client has a '1/measureThroughput' chance to capture the latency of it's next requisition.
	measureThroughput int = 100

	// Just the 'watcherIndex'th client will be recording latency based on 'measureThroughput'.
	watcherIndex int = 0
)

type config struct {
	mustLog     bool
	numKey      int
	numClients  int
	numMessages int
	execTime    int64
}

var Cfg *config

func init() {
	Cfg = new(config)
	flag.IntVar(&Cfg.numClients, "clients", 0, "Set the number of clients")
	flag.IntVar(&Cfg.numMessages, "req", 0, "Set the number of sent requisitions by each client")
	flag.IntVar(&Cfg.numKey, "key", 0, "Set the number of differente keys for hash set")
	flag.Int64Var(&Cfg.execTime, "time", 0, "Set the execution time of the experiment")
	flag.BoolVar(&Cfg.mustLog, "log", true, "Set if this client execution will generate latency logs (0: false; 1: true)")
	flag.IntVar(&dataChoice, "data", -1, "Choose the size of the stored value in the KV storage ('0' = 128B, '1' = 1KB, '2' = 4KB)")
}

func TestNumMessagesKvstore(b *testing.T) {

	b.Parallel()
	flag.Parse()
	if Cfg.numClients == 0 || Cfg.numMessages == 0 || Cfg.numKey == 0 {
		b.Fatal("Must define a number of clients/messages/diff keys > zero")
	}

	switch dataChoice {
	case 0:
		storeValue = oneTweet
		break
	case 1:
		storeValue = oneKB
		break
	case 2:
		storeValue = fourKB
	default:
		log.Fatalf("Must specify a valid option in '-data' argument ('0' = 128B, '1' = 1KB, '2' = 4KB) Passed: %d, %T", dataChoice, dataChoice)
	}

	configBarrier := new(sync.WaitGroup)
	configBarrier.Add(Cfg.numClients)

	finishedBarrier := new(sync.WaitGroup)
	finishedBarrier.Add(Cfg.numClients)

	var logger *log.Logger
	if Cfg.mustLog {
		outFile, err := os.OpenFile(strconv.Itoa(Cfg.numClients)+"c-latency.out", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			b.Fatalf("could not open log file: %s\n", err.Error())
		}
		defer outFile.Close()
		logger = log.New(outFile, "", 0)
	}

	clients := make([]*Info, Cfg.numClients, Cfg.numClients)

	for i := 0; i < Cfg.numClients; i++ {
		go func(j int) {

			runtime.LockOSThread()
			defer runtime.UnlockOSThread()
			chosenClient := j == watcherIndex

			var err error
			clients[j], err = New(*configFilename)
			if err != nil {
				b.Fatalf("failed to find config: %s", err.Error())
			}

			err = clients[j].Connect()
			if err != nil {
				b.Fatalf("failed to connect to cluster: %s", err.Error())
			}

			clients[j].Udpport = 15000 + j
			err = clients[j].StartUDP()
			if err != nil {
				b.Fatalf("failed to start UDP socket: %s", err.Error())
			}

			// Control variables
			var op, coinThroughtput int
			var start time.Time
			var finish int64
			var flagStopwatch bool

			// Wait until all goroutines finish configuration
			configBarrier.Done()
			configBarrier.Wait()

			for k := 0; k < Cfg.numMessages; k++ {

				op = rand.Intn(2)
				if chosenClient && Cfg.mustLog {
					coinThroughtput = rand.Intn(measureThroughput)
					if coinThroughtput == 0 {
						flagStopwatch = true
						start = time.Now()
					}
				}

				var msg *pb.Command
				switch op {
				case 0:
					msg = &pb.Command{
						Op:    pb.Command_SET,
						Key:   strconv.Itoa(rand.Intn(Cfg.numKey)),
						Value: storeValue,
					}
					break

				case 1:
					msg = &pb.Command{
						Op:  pb.Command_GET,
						Key: strconv.Itoa(rand.Intn(Cfg.numKey)),
					}
					break

					// case 2:
					// 	msg = &pb.Command{
					// 		Op:  pb.Command_DELETE,
					// 		Key: strconv.Itoa(rand.Intn(Cfg.numKey)),
					// 	}
				}
				err := clients[j].BroadcastProtobuf(msg, strconv.Itoa(clients[j].Udpport))
				if err != nil {
					b.Logf("Error: %q, caught while broadcasting message: %v", err.Error(), *msg)
				}

				repply, err := clients[j].ReadUDP()
				if err != nil {
					b.Logf("UDP error: %q, caught repply: %s", err.Error(), repply)
				}

				if flagStopwatch {
					finish = int64(time.Since(start) / time.Nanosecond)
					b.Log(finish)
					logger.Println(finish)
					flagStopwatch = false
				}

				thinkTime := clients[j].ThinkingTimeMsec
				if thinkTime > 0 {
					time.Sleep(time.Duration(rand.Intn(thinkTime+1)) * time.Millisecond)
				}
			}
			finishedBarrier.Done()
		}(i)
	}
	finishedBarrier.Wait()

	for _, v := range clients {
		v.Shutdown()
	}
}

func TestClientTimeKvstore(b *testing.T) {

	b.Parallel()
	flag.Parse()
	if Cfg.numClients == 0 || Cfg.execTime == 0 || Cfg.numKey == 0 {
		b.Fatal("Must define a number of clients/execTime/diff keys > zero")
	}

	switch dataChoice {
	case 0:
		storeValue = oneTweet
		break
	case 1:
		storeValue = oneKB
		break
	case 2:
		storeValue = fourKB
	default:
		log.Fatalf("Must specify a valid option in '-data' argument ('0' = 128B, '1' = 1KB, '2' = 4KB) Passed: %d, %T", dataChoice, dataChoice)
	}

	configBarrier := new(sync.WaitGroup)
	configBarrier.Add(Cfg.numClients)

	finishedBarrier := new(sync.WaitGroup)
	finishedBarrier.Add(Cfg.numClients)

	var logger *log.Logger
	if Cfg.mustLog {
		outFile, err := os.OpenFile(strconv.Itoa(Cfg.numClients)+"c-latency.out", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			b.Fatalf("could not open log file: %s\n", err.Error())
		}
		defer outFile.Close()
		logger = log.New(outFile, "", 0)
	}

	clients := make([]*Info, Cfg.numClients, Cfg.numClients)
	signal := make(chan bool)
	requests := make(chan *pb.Command, Cfg.numMessages)

	go generateProtobufRequests(requests, signal, Cfg.numKey, storeValue)
	go killWorkers(Cfg.execTime, signal)

	for i := 0; i < Cfg.numClients; i++ {
		go func(j int, requests chan *pb.Command, kill chan bool) {

			runtime.LockOSThread()
			defer runtime.UnlockOSThread()
			chosenClient := j == watcherIndex

			var err error
			clients[j], err = New(*configFilename)
			if err != nil {
				b.Fatalf("failed to find config: %s", err.Error())
			}

			err = clients[j].Connect()
			if err != nil {
				b.Fatalf("failed to connect to cluster: %s", err.Error())
			}

			clients[j].Udpport = 15000 + j
			err = clients[j].StartUDP()
			if err != nil {
				b.Fatalf("failed to start UDP socket: %s", err.Error())
			}

			// Control variables
			var coinThroughtput int
			var start time.Time
			var finish int64
			var flagStopwatch bool

			// Wait until all goroutines finish configuration
			configBarrier.Done()
			configBarrier.Wait()

			for {
				msg, ok := <-requests
				if !ok {
					finishedBarrier.Done()
					return
				}

				if chosenClient && Cfg.mustLog {
					coinThroughtput = rand.Intn(measureThroughput)
					if coinThroughtput == 0 {
						flagStopwatch = true
						start = time.Now()
					}
				}

				err := clients[j].BroadcastProtobuf(msg, strconv.Itoa(clients[j].Udpport))
				if err != nil {
					b.Logf("Error: %q, caught while broadcasting message: %v", err.Error(), *msg)
				}
				repply, err := clients[j].ReadUDP()
				if err != nil {
					b.Logf("UDP error: %q, caught repply: %s", err.Error(), repply)
				}

				if flagStopwatch {
					finish = int64(time.Since(start) / time.Nanosecond)
					b.Log(finish)
					logger.Println(finish)
					flagStopwatch = false
				}

				thinkTime := clients[j].ThinkingTimeMsec
				if thinkTime > 0 {
					time.Sleep(time.Duration(rand.Intn(thinkTime+1)) * time.Millisecond)
				}
			}

		}(i, requests, signal)
	}
	finishedBarrier.Wait()

	for _, v := range clients {
		v.Shutdown()
	}
}

func generateRequests(reqs chan<- string, signal <-chan bool, numKey int, storeValue string) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	for {
		var msg string
		op := rand.Intn(2)

		switch op {
		case 0:
			msg = fmt.Sprintf("set-%d-%s\n", rand.Intn(numKey), storeValue)
			break
		case 1:
			msg = fmt.Sprintf("get-%d\n", rand.Intn(numKey))
			// case 2:
			// 	msg = fmt.Sprintf("delete-%d\n", rand.Intn(numKey))
		}

		select {
		case reqs <- msg:
			// ...
		case <-signal:
			close(reqs)
			return
		}
	}
}

func generateProtobufRequests(reqs chan<- *pb.Command, signal <-chan bool, numKey int, storeValue string) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	for {
		var msg *pb.Command
		op := rand.Intn(2)

		switch op {
		case 0:
			msg = &pb.Command{
				Op:    pb.Command_SET,
				Key:   strconv.Itoa(rand.Intn(numKey)),
				Value: storeValue,
			}
			break
		case 1:
			msg = &pb.Command{
				Op:  pb.Command_GET,
				Key: strconv.Itoa(rand.Intn(numKey)),
			}
			// case 2:
			// 	msg = &pb.Command{
			// 		Op:  pb.Command_DELETE,
			// 		Key: strconv.Itoa(rand.Intn(numKey)),
			// 	}
		}

		select {
		case reqs <- msg:
			// ...
		case <-signal:
			close(reqs)
			return
		}
	}
}

func killWorkers(timeSeconds int64, signal chan<- bool) {
	t := time.NewTimer(time.Duration(timeSeconds) * time.Second)
	<-t.C
	signal <- true
}
