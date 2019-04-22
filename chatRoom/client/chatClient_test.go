package main

import (
	"math/rand"
	"strconv"
	"sync"
	"testing"
)

func TestTotalOrder(t *testing.T) {

	numClients := 5
	numMessages := 100

	configBarrier := new(sync.WaitGroup)
	configBarrier.Add(numClients)

	finishedBarrier := new(sync.WaitGroup)
	finishedBarrier.Add(numClients)

	for i := 0; i < numClients; i++ {

		go func() {

			// Create some fake data
			data := []int{}
			for j := 0; j < numMessages; j++ {
				data = append(data, rand.Int())
			}

			t.Log("Data configured")

			// Connect to the cluster
			cluster, err := New()
			if err != nil {
				t.Fatalf("failed to find config: %s", err.Error())
			}

			t.Log("rep:", cluster.Rep)
			t.Log("svrIps:", cluster.SvrIps)

			err = cluster.Connect()
			if err != nil {
				t.Fatalf("failed to connect to cluster: %s", err.Error())
			}

			// Wait until all goroutines finish configuration
			configBarrier.Done()
			configBarrier.Wait()

			// Now send requisitions to the cluster
			for _, v := range data {
				cluster.Broadcast(strconv.Itoa(v))
			}

			// TODO:
			// Must implement a way to capture received messages and compared it with sent data[]

			finishedBarrier.Done()
		}()
	}

	finishedBarrier.Wait()
}
