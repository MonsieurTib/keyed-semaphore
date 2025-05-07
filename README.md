![Build](https://github.com/MonsieurTib/keyed-semaphore/actions/workflows/go.yml/badge.svg)
[![Quality Gate Status](https://sonarcloud.io/api/project_badges/measure?project=MonsieurTib_keyed-semaphore&metric=alert_status)](https://sonarcloud.io/summary/new_code?id=MonsieurTib_keyed-semaphore)
[![Go Report Card](https://goreportcard.com/badge/github.com/MonsieurTib/keyed-semaphore)](https://goreportcard.com/report/github.com/MonsieurTib/keyed-semaphore)


# Keyed Semaphore for Go

`keyed-semaphore` provides a semaphore implementation in Go where the locking is based on arbitrary keys (strings). This allows you to limit concurrency for operations associated with specific identifiers, rather than just globally.

For example, you can use it to limit the number of concurrent operations per user ID, per resource ID, or any other string-based key.

## Features

*   **Key-Based Concurrency Limiting:** Limit concurrent access based on string keys.
*   **Configurable Concurrency:** Set the maximum number of concurrent acquirers *per key*.
*   **Context-Aware Waiting:** The `Wait` method respects `context.Context` for cancellation and deadlines.
*   **Non-Blocking TryWait:** A `TryWait` method is available to attempt acquiring the semaphore without blocking.
*   **Dynamic Creation:** Semaphores for keys are created on demand when first accessed.
*   **Automatic Cleanup:** Internal resources associated with a key are cleaned up when the semaphore count for that key drops to zero.

## Installation

```bash
go get github.com/MonsieurTib/keyed-semaphore
```

## Usage

```go
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
	err := semaphore.Wait(resourceID, ctx)
	if err != nil {
		fmt.Printf("Worker %d: Failed lock for resource '%s': %v\n", id, resourceID, err)
		return
	}

	fmt.Printf("Worker %d: Acquired lock for resource '%s'. Working...\n", id, resourceID)

	// Simulate work holding the semaphore
	time.Sleep(time.Second*3)

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

```

## API Overview

*   `NewKeyedSemaphore(maxSize int) *KeyedSemaphore`: Creates a new keyed semaphore manager. `maxSize` defines the maximum concurrent holders for *each* key.
*   `Wait(key string, ctx context.Context) error`: Waits to acquire the semaphore for the given `key`. Blocks until acquisition is possible or the `ctx` is cancelled/times out. Returns `ctx.Err()` on cancellation/timeout.
*   `TryWait(key string) bool`: Attempts to acquire the semaphore for the given `key` without blocking. Returns `true` if successful, `false` otherwise.
*   `Release(key string) error`: Releases the semaphore for the given `key`. Returns an error if the semaphore for the key doesn't exist or if `Release` is called more times than `Wait` for that key.

## Contributing

Contributions are welcome! Please feel free to submit issues and pull requests.

