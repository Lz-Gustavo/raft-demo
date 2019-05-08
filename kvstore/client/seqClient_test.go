package main

import (
	"fmt"
	"math/rand"
	"strconv"
	"sync"
	"testing"
)

func TestRequisitionsKvstore(b *testing.T) {
	numClients := 5
	numMessages := 400

	numKey := 100
	storeValue := "-----"

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

			// TODO: modify this port/addr set
			port := strconv.Itoa(15000 + j)
			clients[j].Udpaddr = "127.0.0.1:" + port
			err = clients[j].StartUDP()
			if err != nil {
				b.Fatalf("failed to start UDP socket: %s", err.Error())
			}

			// Wait until all goroutines finish configuration
			configBarrier.Done()
			configBarrier.Wait()

			for k := 0; k < numMessages; k++ {
				clients[j].Broadcast(fmt.Sprintf("set-%d-%s\n", rand.Intn(numKey), storeValue))
				//clients[j].Broadcast(fmt.Sprintf("get-%d\n", rand.Intn(numKey)))
				//clients[j].Broadcast(fmt.Sprintf("delete-%d\n", rand.Intn(numKey)))

				repply, _ := clients[j].ReadUDP()
				b.Log("Received value:", repply)
			}
			finishedBarrier.Done()
		}(i)
	}
	finishedBarrier.Wait()

	for _, v := range clients {
		v.Shutdown()
	}
}
