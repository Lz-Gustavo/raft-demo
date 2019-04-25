package main

import (
	"errors"
	"sync"
)

// MessageQueue is filled with incoming messages from hosts, ignoring repeated entries
type MessageQueue struct {
	Size int
	Data []string

	Sync  bool
	mutex sync.Mutex
}

// NewMQ initializes a new message queue with defined size and sync configuration
func NewMQ(size int, synchronized bool) *MessageQueue {
	return &MessageQueue{
		Size: size,
		Data: make([]string, size, size),
		Sync: synchronized,
	}
}

// Add method append a new entry to the msgqueue
func (mq *MessageQueue) Add(data string) error {

	if mq.Sync {
		mq.mutex.Lock()
		defer mq.mutex.Unlock()
	}

	if len(mq.Data) >= mq.Size-1 {
		return errors.New("queue already full, consume some data before append")
	}

	if !mq.contains(data) {
		mq.Data = append(mq.Data, data)
		return nil
	}
	return errors.New("equal message already exists in queue, ready to be consumed")
}

// Consume extracts the first element of the queue
func (mq *MessageQueue) Consume() string {

	if mq.Sync {
		mq.mutex.Lock()
		defer mq.mutex.Unlock()
	}

	first := mq.Data[0]
	mq.Data = mq.Data[1:]

	return first
}

// PushPop threats the mq as a restricted size exclusive queue, pushing a element
// to it (in canse it doesnt exists) and removing the first one
func (mq *MessageQueue) PushPop(data string) (string, error) {

	if mq.Sync {
		mq.mutex.Lock()
		defer mq.mutex.Unlock()
	}

	if !mq.contains(data) {

		var aux string
		if len(mq.Data) >= mq.Size-1 {
			aux = mq.Consume()
		}

		mq.Data = append(mq.Data, data)
		return aux, nil
	}
	return "", errors.New("equal message already exists in queue, ready to be consumed")
}

// Contains searches the data structure for a given string, returns true if it exists
func (mq *MessageQueue) contains(data string) bool {

	for _, v := range mq.Data {
		if v == data {
			return true
		}
	}
	return false
}
