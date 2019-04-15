package recovlog

import (
	"sync"
	"testing"
	"time"
)

func TestPutLog(t *testing.T) {

	Logger := New()
	Logger.Put(0, 1, "testing", time.Now().Format(time.Kitchen))
	Logger.Test()
	Logger.Close()
}

func TestBatchUpdate(t *testing.T) {

	Logger := New()

	for i := 0; i < 9; i++ {
		Logger.Put(0, 1, "testing-batch", time.Now().Format(time.Kitchen))
	}

	Logger.Put(0, 1, "testing-final", time.Now().Format(time.Kitchen))
	Logger.Close()
}

func TestBatchConcurrent(t *testing.T) {

	Logger := New()
	var wg sync.WaitGroup
	wg.Add(10)

	for i := 0; i < 10; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < 12; j++ {
				Logger.Put(0, 1, "testing-batch", time.Now().Format(time.Kitchen))
			}
		}()
	}
	wg.Wait()
	Logger.Close()
}

func TestSyncMutex(t *testing.T) {

	Logger := New()
	var wg sync.WaitGroup
	wg.Add(50)

	for i := 0; i < 50; i++ {
		go func() {
			defer wg.Done()
			Logger.Put(0, 1, "testing concurrency", time.Now().Format(time.Kitchen))
		}()
	}
	wg.Wait()
	Logger.Close()
}

// Testing data race on a suddently log.Close() call, without waiting for threads to finish
func TestLogClose(t *testing.T) {

	Logger := New()

	for i := 0; i < 20; i++ {
		go Logger.Put(0, 1, "testing close", time.Now().Format(time.Kitchen))
	}
	Logger.Close()
}

// TODO: must implement the service daemon first
func TestSockets(t *testing.T) {
}
