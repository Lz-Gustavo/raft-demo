package main

import (
	"flag"
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"
)

type config struct {
	configFile  *string
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
	Cfg.configFile = flag.String("testfile", "", "test config toml file")
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
			var finish time.Duration
			var flagStopwatch bool

			// Wait until all goroutines finish configuration
			configBarrier.Done()
			configBarrier.Wait()

			for k := 0; k < numMessages; k++ {

				coinThroughtput = rand.Intn(measureThroughput)
				if coinThroughtput == 0 {
					start = time.Now()
					flagStopwatch = true
				}
				op = rand.Intn(3)
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
					finish = time.Since(start)
					b.Log(finish)
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
