package main

import (
	"fmt"
	"math/rand"
	"sync"
	"testing"
)

func TestRequisitionsKvstore(b *testing.T) {
	numClients := 5
	numMessages := 300

	numKey := 100
	//storeValue := "-----"

	configBarrier := new(sync.WaitGroup)
	configBarrier.Add(numClients)

	finishedBarrier := new(sync.WaitGroup)
	finishedBarrier.Add(numClients)

	clients := make([]*Info, numClients, numClients)

	for i := 0; i < numClients; i++ {
		go func(i int) {

			var err error
			clients[i], err = New("client-config.toml")
			if err != nil {
				b.Fatalf("failed to find config: %s", err.Error())
			}

			err = clients[i].Connect()
			if err != nil {
				b.Fatalf("failed to connect to cluster: %s", err.Error())
			}

			// Wait until all goroutines finish configuration
			configBarrier.Done()
			configBarrier.Wait()

			for j := 0; j < numMessages; j++ {
				//clients[i].Broadcast(fmt.Sprintf("set-%d-%s\n", rand.Intn(numKey), storeValue))
				clients[i].Broadcast(fmt.Sprintf("get-%d\n", rand.Intn(numKey)))
				//clients[i].Broadcast(fmt.Sprintf("delete-%d\n", rand.Intn(numKey)))

				repply := clients[i].ReadParallel()
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
