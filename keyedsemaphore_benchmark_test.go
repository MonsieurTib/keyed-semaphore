package keyedsemaphore

import (
	"context"
	"fmt"
	"math/rand"
	"strconv"
	"testing"
)

func BenchmarkKeyedSemaphore_SingleShard(b *testing.B) {
	sem := NewKeyedSemaphore[string](10)

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			key := "user_" + strconv.Itoa(int(b.N)%4)
			ctx := context.Background()
			if err := sem.Wait(ctx, key); err == nil {
				_ = sem.Release(key)
			}
		}
	})
}

func BenchmarkKeyedSemaphore_Sharded(b *testing.B) {
	shardCount := 16
	sem := NewShardedKeyedSemaphore(shardCount, 10, HashString)

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			key := "user_" + strconv.Itoa(int(b.N)%4)
			ctx := context.Background()
			shard := sem.GetShard(key)
			if err := shard.Wait(ctx, key); err == nil {
				_ = shard.Release(key)
			}
		}
	})
}

const (
	numGoroutines = 1000
	// Number of unique keys (simulating high contention)
	numKeys = 10
	maxSize = 10
)

func BenchmarkKeyedSemaphore_SingleShard_High_Contention(b *testing.B) {
	sem := NewKeyedSemaphore[string](maxSize)
	ctx := context.Background()

	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			// Simulate a high-contention scenario with random keys
			key := fmt.Sprintf("key-%d", rand.Intn(numKeys))
			err := sem.Wait(ctx, key)
			if err != nil {
				b.Fatalf("unexpected error: %v", err)
			}

			// Simulate doing work (e.g., processing the resource)
			// Time.Sleep(time.Millisecond)

			err = sem.Release(key)
			if err != nil {
				b.Fatalf("unexpected error: %v", err)
			}
		}
	})
}

func BenchmarkKeyedSemaphore_Sharded_High_Contention(b *testing.B) {
	shardedSem := NewShardedKeyedSemaphore[string](16, maxSize, HashString)
	ctx := context.Background()

	b.ResetTimer() // Start the timer after setup

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			key := fmt.Sprintf("key-%d", rand.Intn(numKeys))

			shard := shardedSem.GetShard(key)
			err := shard.Wait(ctx, key)
			if err != nil {
				b.Fatalf("unexpected error: %v", err)
			}

			// Time.Sleep(time.Millisecond)

			err = shard.Release(key)
			if err != nil {
				b.Fatalf("unexpected error: %v", err)
			}
		}
	})
}
