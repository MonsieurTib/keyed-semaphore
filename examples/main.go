package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	ks "github.com/MonsieurTib/keyed-semaphore"
)

func worker(id int, resourceID string, semaphore *ks.KeyedSemaphore, wg *sync.WaitGroup) {
	defer wg.Done()
	ctx := context.Background()
	fmt.Printf("Worker %d: Attempting lock for resource '%s'...\n", id, resourceID)

	// Wait blocks until the semaphore for the key can be acquired or context is done.
	err := semaphore.Wait(ctx, resourceID)
	if err != nil {
		fmt.Printf("Worker %d: Failed lock for resource '%s': %v\n", id, resourceID, err)
		return
	}

	fmt.Printf("Worker %d: Acquired lock for resource '%s'. Working...\n", id, resourceID)

	// Simulate work holding the semaphore
	time.Sleep(time.Second * 3)

	fmt.Printf("Worker %d: Work done. Releasing lock for resource '%s'.\n", id, resourceID)

	// Release the semaphore for the key
	err = semaphore.Release(resourceID)
	if err != nil {
		fmt.Printf("Worker %d: Failed to release lock for resource '%s': %v\n", id, resourceID, err)
	}
}

func main() {
	// Create a new KeyedSemaphore, allowing 2 concurrent operations per key.
	maxConcurrentPerKey := 2
	semaphore := ks.NewKeyedSemaphore(maxConcurrentPerKey)

	var wg sync.WaitGroup
	numWorkers := 5
	resourceKey := "document-123"

	fmt.Printf("Starting %d workers for resource '%s' (max %d concurrent)...\n",
		numWorkers, resourceKey, maxConcurrentPerKey)

	// Start multiple workers trying to access the same resource key
	for i := 1; i <= numWorkers; i++ {
		wg.Add(1)
		go worker(i, resourceKey, semaphore, &wg)
	}

	// Start a worker for a different resource key - it won't be blocked by the first set
	wg.Add(1)
	go worker(numWorkers+1, "other-resource-456", semaphore, &wg)

	fmt.Println("Waiting for workers...")
	wg.Wait()
	fmt.Println("All workers finished.")
}
