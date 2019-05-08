package main

import (
	"fmt"
	"math/rand"
	"strings"
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

func TestTotalOrder(t *testing.T) {

	numClients := 1
	numMessages := 100

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

			client, err := New("../client-config.toml")
			if err != nil {
				t.Fatalf("failed to find config: %s", err.Error())
			}

			t.Log("rep:", client.Rep)
			t.Log("svrIps:", client.SvrIps)
			t.Log("mqueueSize:", client.MqueueSize)

			err = client.Connect()
			if err != nil {
				t.Fatalf("failed to connect to cluster: %s", err.Error())
			}

			// Wait until all goroutines finish configuration
			configBarrier.Done()
			configBarrier.Wait()

			// Now send requisitions to the cluster
			for _, v := range data {
				client.Broadcast(v + "\n")
			}

			// Waiting for all repplies, must modify this later
			time.Sleep(3 * time.Second)

			// Compare received with sent messages
			for i, v := range client.Mq.Data {

				realContent := strings.Split(v, "-")[1]
				realContent = strings.TrimSuffix(realContent, "\n")

				if realContent != data[i] {
					t.Log("Messages at index", i, "are diff, (", realContent, "!=", data[i])
					t.Fail()
				}
			}

			client.Shutdown()
			finishedBarrier.Done()
		}()
	}
	finishedBarrier.Wait()
}

func TestRequisitionsKvstore(b *testing.T) {
	numClients := 5
	numMessages := 30

	numKey := 100
	//storeValue := "-----"

	// Create some fake data
	rand.Seed(time.Now().UnixNano())
	data := []string{}

	for j := 0; j < numMessages; j++ {
		data = append(data, randomString(3))
	}
	b.Log("Data configured")

	configBarrier := new(sync.WaitGroup)
	configBarrier.Add(numClients)

	finishedBarrier := new(sync.WaitGroup)
	finishedBarrier.Add(numClients)

	clients := make([]*Info, numClients, numClients)

	for i := 0; i < numClients; i++ {
		go func(i int) {

			var err error
			clients[i], err = New("../client-config.toml")
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
			}

			finishedBarrier.Done()
		}(i)
	}
	finishedBarrier.Wait()

	for _, v := range clients {
		v.Shutdown()
	}
}
