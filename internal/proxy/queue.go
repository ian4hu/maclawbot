package proxy

import (
	"sync"
	"time"
)

// QueueCapacity is the maximum number of messages that can be held in a queue.
// Older messages are dropped when capacity is exceeded.
const QueueCapacity = 200

// MessageQueue is a thread-safe queue for holding iLink messages.
// It supports blocking dequeue for long-poll simulation.
type MessageQueue struct {
	mu       sync.Mutex       // Protects msgs slice
	msgs     []interface{}    // Queued messages
	capacity int              // Maximum queue size
	event    *sync.Cond       // Conditional variable for blocking wait
}

// NewMessageQueue creates a new message queue with default capacity.
func NewMessageQueue() *MessageQueue {
	q := &MessageQueue{
		capacity: QueueCapacity,
		msgs:     make([]interface{}, 0, QueueCapacity),
	}
	q.event = sync.NewCond(&q.mu)
	return q
}

// Enqueue adds a message to the queue.
// If the queue is full, the oldest message is dropped.
func (q *MessageQueue) Enqueue(msg interface{}) {
	q.mu.Lock()
	defer q.mu.Unlock()

	// Drop oldest message if at capacity
	if len(q.msgs) >= q.capacity {
		q.msgs = append(q.msgs[:0], q.msgs[1:]...)
	}
	q.msgs = append(q.msgs, msg)
	q.event.Signal() // Signal waiting dequeue calls
}

// DequeueAll returns all queued messages and clears the queue.
// If timeout > 0, blocks up to timeout duration waiting for messages.
func (q *MessageQueue) DequeueAll(timeout time.Duration) []interface{} {
	deadline := time.Now().Add(timeout)

	q.mu.Lock()
	defer q.mu.Unlock()

	// If timeout specified and queue is empty, wait for signal or timeout
	for timeout > 0 && len(q.msgs) == 0 {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			break // Timeout expired
		}

		// Schedule a wake-up call for the remaining timeout
		timer := time.AfterFunc(remaining, func() {
			q.event.Broadcast()
		})
		
		q.event.Wait() // Wait releases and reacquires the lock
		timer.Stop()
	}

	// Copy current messages and clear queue
	batch := make([]interface{}, len(q.msgs))
	copy(batch, q.msgs)
	q.msgs = q.msgs[:0]
	return batch
}

// Size returns the current number of messages in the queue.
func (q *MessageQueue) Size() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.msgs)
}
