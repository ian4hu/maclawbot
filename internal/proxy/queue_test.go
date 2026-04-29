package proxy

import (
	"testing"
	"time"
)

// TestNewMessageQueue tests queue initialization.
func TestNewMessageQueue(t *testing.T) {
	q := NewMessageQueue()

	if q.capacity != QueueCapacity {
		t.Errorf("Expected capacity %d, got %d", QueueCapacity, q.capacity)
	}

	if len(q.msgs) != 0 {
		t.Errorf("Expected empty queue, got %d messages", len(q.msgs))
	}
}

// TestEnqueueDequeue tests basic enqueue and dequeue operations.
func TestEnqueueDequeue(t *testing.T) {
	q := NewMessageQueue()

	// Enqueue messages
	q.Enqueue("msg1")
	q.Enqueue("msg2")
	q.Enqueue("msg3")

	// Verify size
	if q.Size() != 3 {
		t.Errorf("Expected queue size 3, got %d", q.Size())
	}

	// Dequeue all messages
	msgs := q.DequeueAll(0)

	if len(msgs) != 3 {
		t.Errorf("Expected 3 messages, got %d", len(msgs))
	}

	// Verify messages are in order
	if msgs[0] != "msg1" || msgs[1] != "msg2" || msgs[2] != "msg3" {
		t.Errorf("Messages not in expected order")
	}

	// Queue should be empty
	if q.Size() != 0 {
		t.Errorf("Expected queue to be empty after dequeue, got %d", q.Size())
	}
}

// TestQueueCapacity tests that queue drops oldest messages when full.
func TestQueueCapacity(t *testing.T) {
	q := NewMessageQueue()

	// Fill queue beyond capacity
	for i := 0; i < QueueCapacity+10; i++ {
		q.Enqueue(i)
	}

	// Queue should not exceed capacity
	if q.Size() > QueueCapacity {
		t.Errorf("Queue size %d exceeds capacity %d", q.Size(), QueueCapacity)
	}

	// Dequeue and verify oldest messages were dropped
	msgs := q.DequeueAll(0)
	
	// First message should be the 11th one added (index 10)
	if msgs[0] != 10 {
		t.Errorf("Expected first message to be 10 (oldest kept), got %v", msgs[0])
	}

	// Last message should be the last one added
	expectedLast := QueueCapacity + 9
	if msgs[len(msgs)-1] != expectedLast {
		t.Errorf("Expected last message to be %d, got %v", expectedLast, msgs[len(msgs)-1])
	}
}

// TestDequeueAllWithTimeout_NoWait tests dequeue with timeout=0 returns immediately.
func TestDequeueAllWithTimeout_NoWait(t *testing.T) {
	q := NewMessageQueue()

	// Enqueue a message
	q.Enqueue("test")

	// Dequeue with timeout=0 should return immediately
	start := time.Now()
	msgs := q.DequeueAll(0)
	elapsed := time.Since(start)

	if len(msgs) != 1 {
		t.Errorf("Expected 1 message, got %d", len(msgs))
	}

	// Should complete very quickly (< 10ms)
	if elapsed > 10*time.Millisecond {
		t.Errorf("Dequeue with timeout=0 took too long: %v", elapsed)
	}
}

// TestDequeueAllWithTimeout_WaitsForMessage tests that dequeue blocks until message arrives.
func TestDequeueAllWithTimeout_WaitsForMessage(t *testing.T) {
	q := NewMessageQueue()

	done := make(chan bool)

	// Start dequeue in background (will block waiting for message)
	go func() {
		msgs := q.DequeueAll(5 * time.Second)
		if len(msgs) != 1 || msgs[0] != "delayed" {
			t.Errorf("Expected message 'delayed', got %v", msgs)
		}
		done <- true
	}()

	// Wait a bit then enqueue message
	time.Sleep(50 * time.Millisecond)
	q.Enqueue("delayed")

	// Should complete successfully
	select {
	case <-done:
		// Success
	case <-time.After(2 * time.Second):
		t.Fatal("Dequeue timed out waiting for message")
	}
}

// TestDequeueAllWithTimeout_Timeout tests that dequeue returns nil on timeout.
func TestDequeueAllWithTimeout_Timeout(t *testing.T) {
	q := NewMessageQueue()

	// Dequeue with short timeout on empty queue
	start := time.Now()
	msgs := q.DequeueAll(100 * time.Millisecond)
	elapsed := time.Since(start)

	// Should return nil (or empty slice)
	if len(msgs) != 0 {
		t.Errorf("Expected 0 messages on timeout, got %d", len(msgs))
	}

	// Should timeout after approximately 100ms
	if elapsed < 90*time.Millisecond || elapsed > 200*time.Millisecond {
		t.Errorf("Timeout duration incorrect: expected ~100ms, got %v", elapsed)
	}
}

// TestConcurrentEnqueueDequeue tests concurrent access (no race conditions).
func TestConcurrentEnqueueDequeue(t *testing.T) {
	q := NewMessageQueue()

	done := make(chan bool)

	// Multiple producers
	for i := 0; i < 5; i++ {
		go func(id int) {
			for j := 0; j < 20; j++ {
				q.Enqueue(id*100 + j)
			}
			done <- true
		}(i)
	}

	// Multiple consumers
	for i := 0; i < 2; i++ {
		go func() {
			for j := 0; j < 10; j++ {
				q.DequeueAll(10 * time.Millisecond)
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 7; i++ {
		<-done
	}
}

// TestQueueSize tests the Size method.
func TestQueueSize(t *testing.T) {
	q := NewMessageQueue()

	if q.Size() != 0 {
		t.Errorf("Expected size 0, got %d", q.Size())
	}

	q.Enqueue("msg1")
	q.Enqueue("msg2")

	if q.Size() != 2 {
		t.Errorf("Expected size 2, got %d", q.Size())
	}

	q.DequeueAll(0)

	if q.Size() != 0 {
		t.Errorf("Expected size 0 after dequeue, got %d", q.Size())
	}
}
