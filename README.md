![Build](https://github.com/MonsieurTib/keyed-semaphore/actions/workflows/go.yml/badge.svg)
[![Quality Gate Status](https://sonarcloud.io/api/project_badges/measure?project=MonsieurTib_keyed-semaphore&metric=alert_status)](https://sonarcloud.io/summary/new_code?id=MonsieurTib_keyed-semaphore)
[![Go Report Card](https://goreportcard.com/badge/github.com/MonsieurTib/keyed-semaphore)](https://goreportcard.com/report/github.com/MonsieurTib/keyed-semaphore)


# Keyed Semaphore for Go

`keyed-semaphore` provides a semaphore implementation in Go where the locking is based on arbitrary keys. This allows you to limit concurrency for operations associated with specific identifiers, rather than just globally.
For example, you can use it to limit the number of concurrent operations per user ID, per resource ID, or any other comparable key type.

## Features

*   **Key-Based Concurrency Limiting:** Limit concurrent access based on generic keys (any Go `comparable` type).
*   **Sharded Implementation for Enhanced Scalability:** Includes a `ShardedKeyedSemaphore` that distributes keys across multiple internal `KeyedSemaphore` instances. This significantly reduces lock contention and improves performance, especially under high load with many unique keys.
*   **Configurable Concurrency:** Set the maximum number of concurrent acquirers *per key*.
*   **Context-Aware Waiting:** The `Wait` method respects `context.Context` for cancellation and deadlines.
*   **Non-Blocking TryWait:** A `TryWait` method is available to attempt acquiring the semaphore without blocking, also respecting `context.Context`.
*   **Dynamic Creation:** Semaphores for keys are created on demand when first accessed.
*   **Robust Memory Management:** Implemented reference counting for automatic and safe cleanup of internal resources associated with a key when it's no longer in active use, preventing memory leaks.
*   **Improved Concurrency Safety:** Hardened against race conditions for reliable behavior under high concurrent access.

## Installation

```bash
go get github.com/MonsieurTib/keyed-semaphore
```

## Usage

### Basic KeyedSemaphore

The following example demonstrates using `KeyedSemaphore` with string keys. Note that `KeyedSemaphore` now supports generic key types.

```go
package main

import (
	"context"
	"fmt"
	ks "github.com/MonsieurTib/keyed-semaphore"
	"sync"
	"time"
)

func worker(id int, resourceID string, semaphore *ks.KeyedSemaphore[string], wg *sync.WaitGroup) {
	defer wg.Done()
	ctx := context.Background()
	fmt.Printf("Worker %d: Attempting lock for resource '%s'...\n", id, resourceID)

	err := semaphore.Wait(ctx, resourceID) 
	if err != nil {
		fmt.Printf("Worker %d: Failed lock for resource '%s': %v\n", id, resourceID, err)
		return
	}

	fmt.Printf("Worker %d: Acquired lock for resource '%s'. Working...\n", id, resourceID)
	time.Sleep(time.Second * 1) // Simulate work
	fmt.Printf("Worker %d: Work done. Releasing lock for resource '%s'.\n", id, resourceID)

	err = semaphore.Release(resourceID)
	if err != nil {
		fmt.Printf("Worker %d: Failed to release lock for resource '%s': %v\n", id, resourceID, err)
	}
}

func main() {
	// Create a new KeyedSemaphore for string keys, allowing 2 concurrent operations per key.
	maxConcurrentPerKey := 2
	semaphore := ks.NewKeyedSemaphore[string](maxConcurrentPerKey)

	var wg sync.WaitGroup
	numWorkers := 5
	resourceKey1 := "document-123"

	fmt.Printf("Starting %d workers for resource '%s' (max %d concurrent)...\n",
		numWorkers, resourceKey1, maxConcurrentPerKey)

	for i := 1; i <= numWorkers; i++ {
		wg.Add(1)
		go worker(i, resourceKey1, semaphore, &wg)
	}

	// Example with a different key type (if you were using int keys)
	// type UserID int
	// userIDKey := UserID(42)
	// semaphoreInt := ks.NewKeyedSemaphore[UserID](maxConcurrentPerKey)
	// ... then use semaphoreInt with userIDKey ...

	wg.Wait()
	fmt.Println("All workers finished for resourceKey1.")
}

```

### Sharded KeyedSemaphore for Higher Concurrency

For scenarios with a very large number of unique keys or extremely high contention, `ShardedKeyedSemaphore` can provide better performance by dividing keys among several independent `KeyedSemaphore` instances (shards).

```go
package main

import (
	"context"
	"fmt"
	ks "github.com/MonsieurTib/keyed-semaphore" 
	"strconv"
	"sync"
	"time"
)

func shardedWorker(id int, resourceID string, shardedSem *ks.ShardedKeyedSemaphore[string], wg *sync.WaitGroup) {
	defer wg.Done()
	ctx := context.Background()
	fmt.Printf("ShardedWorker %d: Attempting lock for resource '%s'...\n", id, resourceID)
	shard := shardedSem.GetShard(resourceID)
	err := shard.Wait(ctx, resourceID)
	if err != nil {
		fmt.Printf("ShardedWorker %d: Failed lock for resource '%s': %v\n", id, resourceID, err)
		return
	}

	fmt.Printf("ShardedWorker %d: Acquired lock for resource '%s'. Working...\n", id, resourceID)
	time.Sleep(time.Millisecond * 500) // Simulate work
	fmt.Printf("ShardedWorker %d: Work done. Releasing lock for resource '%s'.\n", id, resourceID)

	err = shard.Release(resourceID) 
	if err != nil {
		fmt.Printf("ShardedWorker %d: Failed to release lock for resource '%s': %v\n", id, resourceID, err)
	}
}

func main() {
	shardCount := 16 // Number of internal shards
	maxConcurrentPerKey := 2

	// For string keys, you can use the provided HashString function or your own.
	shardedSemaphore := ks.NewShardedKeyedSemaphore[string](shardCount, maxConcurrentPerKey, ks.HashString)

	var wg sync.WaitGroup
	numWorkers := 20 

	fmt.Printf("Starting %d sharded workers (max %d concurrent per key, across %d shards)...\n",
		numWorkers, maxConcurrentPerKey, shardCount)

	for i := 1; i <= numWorkers; i++ {
		wg.Add(1)
		// Simulate different keys that might fall into different shards
		resourceKey := "item-" + strconv.Itoa(i%5)

		// Pass the main shardedSemaphore instance to the worker
		go shardedWorker(i, resourceKey, shardedSemaphore, &wg)
	}

	wg.Wait()
	fmt.Println("All sharded workers finished.")
}
```

## API Overview

### Type Definition for Hasher (used by ShardedKeyedSemaphore)
`type Hasher[K comparable] func(K) uint64`
  * A function type that takes a key of generic type `K` and returns a `uint64` hash value.
  * `HashString(key string) uint64` is provided as a convenient hasher for string keys using `xxhash`.

### `KeyedSemaphore[K comparable]`

*   `NewKeyedSemaphore[K comparable](maxSize int) *KeyedSemaphore[K]`: Creates a new keyed semaphore manager. `maxSize` defines the maximum concurrent holders for *each* key. `K` can be any Go `comparable` type.
*   `Wait(ctx context.Context, key K) error`: Waits to acquire the semaphore for the given `key`. Blocks until acquisition is possible or the `ctx` is cancelled/times out. Returns `ctx.Err()` on cancellation/timeout.
*   `TryWait(ctx context.Context, key K) bool`: Attempts to acquire the semaphore for the given `key` without blocking. Respects `context.Context` for early cancellation. Returns `true` if successful, `false` otherwise.
*   `Release(key K) error`: Releases the semaphore for the given `key`. Returns an error if the semaphore for the key was not previously acquired or if issues occur during release.

### `ShardedKeyedSemaphore[K comparable]`

*   `NewShardedKeyedSemaphore[K comparable](shardCount, maxSize int, hasher Hasher[K]) *ShardedKeyedSemaphore[K]`: Creates a new sharded keyed semaphore manager.
    *   `shardCount`: The number of internal `KeyedSemaphore` instances (shards).
    *   `maxSize`: The maximum concurrent holders for *each* key within its designated shard.
    *   `hasher`: A `Hasher[K]` function to distribute keys among shards.
*   `(sks *ShardedKeyedSemaphore[K]) GetShard(key K) *KeyedSemaphore[K]`: Returns the specific `KeyedSemaphore` instance (shard) responsible for the given `key`. Operations (`Wait`, `TryWait`, `Release`) should then be called on this returned shard.

## Contributing

Contributions are welcome! Please feel free to submit issues and pull requests.

