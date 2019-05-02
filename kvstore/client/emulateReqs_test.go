package main

import (
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
	numClients := 4
	numMessages := 10000

	// Create some fake data
	rand.Seed(time.Now().UnixNano())
	data := []string{}

	for j := 0; j < numMessages; j++ {
		data = append(data, randomString(10))
	}
	t.Log("Data configured")

	configBarrier := new(sync.WaitGroup)
	configBarrier.Add(numClients)

	finishedBarrier := new(sync.WaitGroup)
	finishedBarrier.Add(numClients)

	for i := 0; i < numClients; i++ {

		go func() {

			// TODO: Connect to the cluster

			// Wait until all goroutines finish configuration
			configBarrier.Done()
			configBarrier.Wait()

			// TODO: Send requisitions to the servers

			finishedBarrier.Done()
		}()
	}
	finishedBarrier.Wait()
}
