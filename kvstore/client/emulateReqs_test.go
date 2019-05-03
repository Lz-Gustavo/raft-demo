package main

import (
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"
)

func randomString(len int) string {

	bytes := make([]byte, len)
	for i := 0; i < len; i++ {
		bytes[i] = byte(65 + rand.Intn(90-65))
	}

	return string(bytes)
}

func TestRequisitions(t *testing.T) {
	numClients := 100
	numMessages := 20000

	// Create some fake data
	rand.Seed(time.Now().UnixNano())
	data := []string{}

	for j := 0; j < numMessages; j++ {
		data = append(data, randomString(5))
	}
	t.Log("Data configured")

	configBarrier := new(sync.WaitGroup)
	configBarrier.Add(numClients)

	finishedBarrier := new(sync.WaitGroup)
	finishedBarrier.Add(numClients)

	clients := make([]*Info, numClients, numClients)

	for i := 0; i < numClients; i++ {

		go func(i int) {

			var err error
			clients[i], err = New()
			if err != nil {
				t.Fatalf("failed to find config: %s", err.Error())
			}

			err = clients[i].Connect()
			if err != nil {
				t.Fatalf("failed to connect to cluster: %s", err.Error())
			}

			// Wait until all goroutines finish configuration
			configBarrier.Done()
			configBarrier.Wait()

			// TODO: Send requisitions to the servers
			for _, v := range data {
				clients[i].Broadcast(fmt.Sprintf("set-%s-%s\n", v, v))
			}

			finishedBarrier.Done()
		}(i)
	}
	finishedBarrier.Wait()

	for _, v := range clients {
		v.Shutdown()
	}
}
