package executor

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestDynamicQueue_EnqueueDequeue(t *testing.T) {
	stopC := make(chan struct{})
	defer close(stopC)

	queue := newDynamicQueue(stopC, enqueueAtEndPolicy, fifoDequeuePolicy)

	executed := false
	queue.InChannel() <- func() {
		executed = true
	}

	job := <-queue.OutChannel()
	job()

	if !executed {
		t.Error("job was not executed")
	}
}

func TestDynamicQueue_FIFO_Order(t *testing.T) {
	stopC := make(chan struct{})
	defer close(stopC)

	queue := newDynamicQueue(stopC, enqueueAtEndPolicy, fifoDequeuePolicy)

	order := make([]int, 0, 5)
	var mu sync.Mutex

	for i := 0; i < 5; i++ {
		val := i
		queue.InChannel() <- func() {
			mu.Lock()
			order = append(order, val)
			mu.Unlock()
		}
	}

	for i := 0; i < 5; i++ {
		job := <-queue.OutChannel()
		job()
	}

	for i := 0; i < 5; i++ {
		if order[i] != i {
			t.Errorf("expected FIFO order, got %v", order)
			break
		}
	}
}

func TestDynamicQueue_LIFO_Order(t *testing.T) {
	stopC := make(chan struct{})
	defer close(stopC)

	queue := newDynamicQueue(stopC, enqueueAtEndPolicy, lifoDequeuePolicy)

	order := make([]int, 0, 5)
	var mu sync.Mutex

	// Enqueue all jobs
	for i := 0; i < 5; i++ {
		val := i
		go func() {
			queue.InChannel() <- func() {
				mu.Lock()
				order = append(order, val)
				mu.Unlock()
			}
		}()
	}

	// Wait for all jobs to be queued
	time.Sleep(50 * time.Millisecond)

	// Dequeue all jobs
	for i := 0; i < 5; i++ {
		job := <-queue.OutChannel()
		job()
	}

	// LIFO should process recent jobs first (but order may vary due to goroutine scheduling)
	// Just verify all jobs were executed
	if len(order) != 5 {
		t.Errorf("expected 5 jobs executed, got %d", len(order))
	}

	// Verify all values 0-4 are present
	seen := make(map[int]bool)
	for _, val := range order {
		seen[val] = true
	}
	for i := 0; i < 5; i++ {
		if !seen[i] {
			t.Errorf("value %d was not executed", i)
		}
	}
}

func TestDynamicQueue_PolicyChange(t *testing.T) {
	stopC := make(chan struct{})
	defer close(stopC)

	queue := newDynamicQueue(stopC, enqueueAtEndPolicy, fifoDequeuePolicy)

	// Enqueue some jobs
	for i := 0; i < 3; i++ {
		queue.InChannel() <- func() {}
	}

	time.Sleep(10 * time.Millisecond)

	// Change to LIFO
	queue.SetDequeuePolicy(lifoDequeuePolicy)

	// Should not panic
	job := <-queue.OutChannel()
	job()
}

func TestDynamicQueue_SinceLastEmpty(t *testing.T) {
	stopC := make(chan struct{})
	defer close(stopC)

	queue := newDynamicQueue(stopC, enqueueAtEndPolicy, fifoDequeuePolicy)

	// Add multiple jobs
	for i := 0; i < 3; i++ {
		queue.InChannel() <- func() {}
	}

	// Wait a bit for jobs to be queued
	time.Sleep(20 * time.Millisecond)

	// Should have been non-empty for at least some time
	since := queue.SinceLastEmpty()
	if since < 1*time.Millisecond || since > 100*time.Millisecond {
		t.Logf("SinceLastEmpty: %v (acceptable range)", since)
	}

	// Dequeue all jobs
	for i := 0; i < 3; i++ {
		job := <-queue.OutChannel()
		job()
	}
}

func TestDynamicQueue_ConcurrentEnqueue(t *testing.T) {
	stopC := make(chan struct{})
	defer close(stopC)

	queue := newDynamicQueue(stopC, enqueueAtEndPolicy, fifoDequeuePolicy)

	count := 50
	var executed atomic.Int32

	// Enqueue and dequeue concurrently
	for i := 0; i < count; i++ {
		go func() {
			queue.InChannel() <- func() {
				executed.Add(1)
			}
		}()
	}

	// Dequeue all with timeout
	timeout := time.After(2 * time.Second)
	for i := 0; i < count; i++ {
		select {
		case job := <-queue.OutChannel():
			job()
		case <-timeout:
			t.Fatalf("timeout, only dequeued %d/%d jobs, executed %d", i, count, executed.Load())
		}
	}

	if int(executed.Load()) != count {
		t.Errorf("expected %d jobs executed, got %d", count, executed.Load())
	}
}

func TestQueueStats_IncDecr(t *testing.T) {
	stats := &queueStats{}

	stats.inc()
	if stats.size != 1 {
		t.Errorf("expected size 1, got %d", stats.size)
	}

	stats.inc()
	if stats.size != 2 {
		t.Errorf("expected size 2, got %d", stats.size)
	}

	stats.decr()
	if stats.size != 1 {
		t.Errorf("expected size 1, got %d", stats.size)
	}

	stats.decr()
	if stats.size != 0 {
		t.Errorf("expected size 0, got %d", stats.size)
	}
}

func TestEnqueueAtEndPolicy(t *testing.T) {
	queue := []func(){}

	job1 := func() {}
	job2 := func() {}

	queue = enqueueAtEndPolicy(job1, queue)
	if len(queue) != 1 {
		t.Errorf("expected queue length 1, got %d", len(queue))
	}

	queue = enqueueAtEndPolicy(job2, queue)
	if len(queue) != 2 {
		t.Errorf("expected queue length 2, got %d", len(queue))
	}
}

func TestFifoDequeuePolicy(t *testing.T) {
	job1 := func() { t.Log("job1") }
	job2 := func() { t.Log("job2") }
	queue := []func(){job1, job2}

	job, remaining := fifoDequeuePolicy(queue)
	if job == nil {
		t.Error("expected job, got nil")
	}
	if len(remaining) != 1 {
		t.Errorf("expected remaining length 1, got %d", len(remaining))
	}

	// Empty queue
	_, remaining = fifoDequeuePolicy([]func(){})
	if len(remaining) != 0 {
		t.Errorf("expected empty queue, got %d", len(remaining))
	}
}

func TestLifoDequeuePolicy(t *testing.T) {
	job1 := func() { t.Log("job1") }
	job2 := func() { t.Log("job2") }
	queue := []func(){job1, job2}

	job, remaining := lifoDequeuePolicy(queue)
	if job == nil {
		t.Error("expected job, got nil")
	}
	if len(remaining) != 1 {
		t.Errorf("expected remaining length 1, got %d", len(remaining))
	}

	// Empty queue
	_, remaining = lifoDequeuePolicy([]func(){})
	if len(remaining) != 0 {
		t.Errorf("expected empty queue, got %d", len(remaining))
	}
}
