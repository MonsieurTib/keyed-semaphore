package main

import (
	"context"
	"fmt"
	ks "github.com/MonsieurTib/keyed-semaphore"
	"sync"
	"time"
)

func worker(ctx context.Context, id int, resourceID string, semaphore *ks.KeyedSemaphore[string], wg *sync.WaitGroup) {
	defer wg.Done()
	fmt.Printf("Worker %d: Attempting lock for resource '%s'...\n", id, resourceID)
	var subWg sync.WaitGroup
	for range 50 {
		subWg.Add(1)
		go func() {
			defer subWg.Done()
			err := semaphore.Wait(ctx, resourceID)
			if err != nil {
				panic(
					fmt.Errorf(
						"worker %d: failed to acquire lock for resource %q: %w",
						id,
						resourceID,
						err,
					),
				)
			}
			// fmt.Printf("Worker %d: Acquired lock for resource '%s'. Working...\n", id, resourceID)
			// Simulate work holding the semaphore
			time.Sleep(time.Millisecond * 1)
			// fmt.Printf("Worker %d: Work done. Releasing lock for resource '%s'.\n", id, resourceID)
			// Release the semaphore for the key
			err = semaphore.Release(resourceID)
			if err != nil {
				panic(
					fmt.Errorf(
						"worker %d: failed to release lock for resource %q: %w",
						id,
						resourceID,
						err,
					),
				)
			}
		}()
	}
	subWg.Wait()
}

func main() {
	maxConcurrentPerKey := 10
	semaphore := ks.NewKeyedSemaphore[string](maxConcurrentPerKey)

	ctx := context.Background()
	var wg sync.WaitGroup
	numWorkers := 1000
	resourceKey := "document-123"

	fmt.Printf("Starting %d workers for resource '%s' (max %d concurrent)...\n",
		numWorkers, resourceKey, maxConcurrentPerKey)

	for i := 1; i <= numWorkers; i++ {
		wg.Add(1)
		go worker(ctx, i, resourceKey, semaphore, &wg)
	}

	wg.Add(1)
	go worker(ctx, numWorkers+1, "other-resource-456", semaphore, &wg)

	fmt.Println("Waiting for workers...")
	wg.Wait()
	fmt.Println("All workers finished.")
	fmt.Printf("Final semaphore state: %v\n", semaphore)
}
