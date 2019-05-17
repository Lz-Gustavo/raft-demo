package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"
)

type config struct {
	numKey      int
	numClients  int
	numMessages int
	execTime    int64
}

var Cfg *config

const storeValue = "@@@@@"

func init() {
	Cfg = new(config)
	flag.IntVar(&Cfg.numClients, "clients", 0, "Set the number of clients")
	flag.IntVar(&Cfg.numMessages, "req", 0, "Set the number of sent requisitions by each client")
	flag.IntVar(&Cfg.numKey, "key", 0, "Set the number of differente keys for hash set")
	flag.Int64Var(&Cfg.execTime, "time", 0, "Set the execution time of the experiment")
}

func TestNumMessagesKvstore(b *testing.T) {

	flag.Parse()
	if Cfg.numClients == 0 || Cfg.numMessages == 0 || Cfg.numKey == 0 {
		b.Fatal("Must define a number of clients/messages/diff keys > zero")
	}
	measureThroughput := 50

	configBarrier := new(sync.WaitGroup)
	configBarrier.Add(Cfg.numClients)

	finishedBarrier := new(sync.WaitGroup)
	finishedBarrier.Add(Cfg.numClients)

	outFile, err := os.OpenFile(strconv.Itoa(Cfg.numClients)+"c-latency.txt", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		b.Fatalf("could not open log file: %s\n", err.Error())
	}
	defer outFile.Close()
	logger := log.New(outFile, "", 0)

	clients := make([]*Info, Cfg.numClients, Cfg.numClients)

	for i := 0; i < Cfg.numClients; i++ {
		go func(j int) {

			var err error
			clients[j], err = New("client-config.toml")
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

				op = rand.Intn(3)
				coinThroughtput = rand.Intn(measureThroughput)
				if coinThroughtput == 0 {
					flagStopwatch = true
					start = time.Now()
				}

				switch op {
				case 0:
					clients[j].Broadcast(fmt.Sprintf("set-%d-%s\n", rand.Intn(Cfg.numKey), storeValue))
				case 1:
					clients[j].Broadcast(fmt.Sprintf("get-%d\n", rand.Intn(Cfg.numKey)))
				case 2:
					clients[j].Broadcast(fmt.Sprintf("delete-%d\n", rand.Intn(Cfg.numKey)))
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

	flag.Parse()
	if Cfg.numClients == 0 || Cfg.execTime == 0 || Cfg.numKey == 0 {
		b.Fatal("Must define a number of clients/execTime/diff keys > zero")
	}
	measureThroughput := 50

	configBarrier := new(sync.WaitGroup)
	configBarrier.Add(Cfg.numClients)

	finishedBarrier := new(sync.WaitGroup)
	finishedBarrier.Add(Cfg.numClients)

	outFile, err := os.OpenFile(strconv.Itoa(Cfg.numClients)+"c-latency.txt", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		b.Fatalf("could not open log file: %s\n", err.Error())
	}
	defer outFile.Close()
	logger := log.New(outFile, "", 0)

	clients := make([]*Info, Cfg.numClients, Cfg.numClients)
	signal := make(chan bool)
	requests := make(chan string, Cfg.numMessages)

	go generateRequests(requests, signal, Cfg.numKey, storeValue)
	go killWorkers(Cfg.execTime, signal)

	for i := 0; i < Cfg.numClients; i++ {
		go func(j int, requests chan string, kill chan bool) {

			var err error
			clients[j], err = New("client-config.toml")
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

				coinThroughtput = rand.Intn(measureThroughput)
				if coinThroughtput == 0 {
					flagStopwatch = true
					start = time.Now()
				}

				clients[j].Broadcast(msg)
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
			}

		}(i, requests, signal)
	}
	finishedBarrier.Wait()

	for _, v := range clients {
		v.Shutdown()
	}
}

func generateRequests(reqs chan<- string, signal <-chan bool, numKey int, storeValue string) {
	for {
		var msg string
		op := rand.Intn(3)

		switch op {
		case 0:
			msg = fmt.Sprintf("set-%d-%s\n", rand.Intn(numKey), storeValue)
		case 1:
			msg = fmt.Sprintf("get-%d\n", rand.Intn(numKey))
		case 2:
			msg = fmt.Sprintf("delete-%d\n", rand.Intn(numKey))
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
