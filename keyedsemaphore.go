package keyedsemaphore

import (
	"context"
	"fmt"
	"github.com/cespare/xxhash"
	"sync"
	"sync/atomic"
)

type Hasher[K comparable] func(K) uint64

type ShardedKeyedSemaphore[K comparable] struct {
	shards []*KeyedSemaphore[K]
	hasher Hasher[K]
}

type semaphore[K comparable] struct {
	ch       chan struct{}
	refCount int32 // Number of active users (Wait successful, Release not yet called)
}

type KeyedSemaphore[K comparable] struct {
	maxSize int
	semMap  map[K]*semaphore[K]
	mu      sync.RWMutex
}

func NewShardedKeyedSemaphore[K comparable](
	shardCount, maxSize int,
	hasher Hasher[K],
) *ShardedKeyedSemaphore[K] {
	shards := make([]*KeyedSemaphore[K], shardCount)
	for i := range shards {
		shards[i] = NewKeyedSemaphore[K](maxSize)
	}
	return &ShardedKeyedSemaphore[K]{
		shards: shards,
		hasher: hasher,
	}
}

func (sks *ShardedKeyedSemaphore[K]) GetShard(key K) *KeyedSemaphore[K] {
	hash := sks.hasher(key)
	return sks.shards[hash%uint64(len(sks.shards))]
}

func HashString(key string) uint64 {
	return xxhash.Sum64String(key)
}

func NewKeyedSemaphore[K comparable](maxSize int) *KeyedSemaphore[K] {
	return &KeyedSemaphore[K]{
		maxSize: maxSize,
		semMap:  make(map[K]*semaphore[K]),
	}
}

func (ks *KeyedSemaphore[K]) Wait(ctx context.Context, key K) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	var sem *semaphore[K]

	ks.mu.Lock()
	s, exists := ks.semMap[key]
	if exists {
		sem = s
		atomic.AddInt32(&sem.refCount, 1)
	} else {
		select {
		case <-ctx.Done():
			ks.mu.Unlock()
			return ctx.Err()
		default:
		}
		sem = &semaphore[K]{ch: make(chan struct{}, ks.maxSize), refCount: 1}
		ks.semMap[key] = sem
	}
	ks.mu.Unlock()
	select {
	case sem.ch <- struct{}{}:
		return nil // Acquired successfully
	case <-ctx.Done():
		ks.mu.Lock()
		newRefCount := atomic.AddInt32(&sem.refCount, -1)
		if newRefCount == 0 && len(sem.ch) == 0 {
			if currentHolderInMap, ok := ks.semMap[key]; ok && currentHolderInMap == sem {
				delete(ks.semMap, key)
			}
		}
		ks.mu.Unlock()
		return ctx.Err()
	}
}

func (ks *KeyedSemaphore[K]) TryWait(ctx context.Context, key K) bool {
	select {
	case <-ctx.Done():
		return false
	default:
	}

	var sem *semaphore[K]

	ks.mu.Lock()
	s, exists := ks.semMap[key]
	if exists {
		sem = s
		atomic.AddInt32(&sem.refCount, 1)
	} else {
		select {
		case <-ctx.Done():
			ks.mu.Unlock()
			return false
		default:
		}
		sem = &semaphore[K]{ch: make(chan struct{}, ks.maxSize), refCount: 1}
		ks.semMap[key] = sem
	}
	ks.mu.Unlock()

	select {
	case sem.ch <- struct{}{}:
		return true // Acquired successfully
	case <-ctx.Done():
		ks.mu.Lock()
		newRefCount := atomic.AddInt32(&sem.refCount, -1)
		if newRefCount == 0 && len(sem.ch) == 0 {
			if currentHolderInMap, ok := ks.semMap[key]; ok && currentHolderInMap == sem {
				delete(ks.semMap, key)
			}
		}
		ks.mu.Unlock()
		return false
	default: // Channel is full, TryWait fails
		ks.mu.Lock()
		newRefCount := atomic.AddInt32(&sem.refCount, -1)
		if newRefCount == 0 && len(sem.ch) == 0 { // Channel empty
			if currentHolderInMap, ok := ks.semMap[key]; ok && currentHolderInMap == sem {
				delete(ks.semMap, key)
			}
		}
		ks.mu.Unlock()
		return false
	}
}

func (ks *KeyedSemaphore[K]) Release(key K) error {
	ks.mu.RLock()
	s, exists := ks.semMap[key]
	ks.mu.RUnlock()

	if !exists {
		return fmt.Errorf(
			"attempting to release a semaphore that doesn't exist for key: %v (not found in map or never waited on)",
			key,
		)
	}

	select {
	case <-s.ch:
		// Permit successfully released from channel
		ks.mu.Lock()
		newRefCount := atomic.AddInt32(&s.refCount, -1)

		if newRefCount < 0 {
			atomic.AddInt32(&s.refCount, 1)
			ks.mu.Unlock()
			return fmt.Errorf(
				"semaphore for key %v released more times than acquired (refCount became negative)",
				key,
			)
		}

		if newRefCount == 0 && len(s.ch) == 0 {
			if currentHolderInMap, ok := ks.semMap[key]; ok && currentHolderInMap == s {
				delete(ks.semMap, key)
			}
		}
		ks.mu.Unlock()
		return nil
	default:
		// Channel was empty, implying a Release without a corresponding successful Wait or a double Release.
		return fmt.Errorf(
			"release called on an already empty semaphore channel for key: %v (potential double release or release without wait)",
			key,
		)
	}
}
