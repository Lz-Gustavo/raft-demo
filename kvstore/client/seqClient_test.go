package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"sync"
	"testing"
	"time"
)

type config struct {
	numKey      int
	numClients  int
	numMessages int
}

var Cfg *config

func init() {
	Cfg = new(config)
	flag.IntVar(&Cfg.numClients, "clients", 0, "Set the number of clients")
	flag.IntVar(&Cfg.numMessages, "req", 0, "Set the number of sent requisitions by each client")
	flag.IntVar(&Cfg.numKey, "key", 0, "Set the number of differente keys for hash set")
}

func TestKvstore(b *testing.T) {

	flag.Parse()
	if Cfg.numClients == 0 || Cfg.numMessages == 0 || Cfg.numKey == 0 {
		b.Fatal("Must define a number of clients/messages/diff keys > zero")
	}
	numClients := Cfg.numClients
	numMessages := Cfg.numMessages

	numKey := Cfg.numKey
	storeValue := "-----"

	measureThroughput := 50

	configBarrier := new(sync.WaitGroup)
	configBarrier.Add(numClients)

	finishedBarrier := new(sync.WaitGroup)
	finishedBarrier.Add(numClients)

	outFile, err := os.OpenFile("testLatency.txt", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		b.Fatalf("could not open log file: %s\n", err.Error())
	}
	defer outFile.Close()
	logger := log.New(outFile, "", 0)

	clients := make([]*Info, numClients, numClients)

	for i := 0; i < numClients; i++ {
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

			for k := 0; k < numMessages; k++ {

				op = rand.Intn(3)
				coinThroughtput = rand.Intn(measureThroughput)
				if coinThroughtput == 0 {
					flagStopwatch = true
					start = time.Now()
				}

				switch op {
				case 0:
					clients[j].Broadcast(fmt.Sprintf("set-%d-%s\n", rand.Intn(numKey), storeValue))
				case 1:
					clients[j].Broadcast(fmt.Sprintf("get-%d\n", rand.Intn(numKey)))
				case 2:
					clients[j].Broadcast(fmt.Sprintf("delete-%d\n", rand.Intn(numKey)))
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
