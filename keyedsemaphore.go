package keyedsemaphore

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"hash/fnv"
	"log"
	"sync"
)

type ShardedKeyedSemaphore[K comparable] []*KeyedSemaphore[K]

type KeyedSemaphore[K comparable] struct {
	maxSize int
	semMap  map[K]chan struct{}
	mu      sync.RWMutex
}

func NewShardedKeyedSemaphore[K comparable](shardCount, maxSize int) ShardedKeyedSemaphore[K] {
	sem := make(ShardedKeyedSemaphore[K], shardCount)
	for i := range shardCount {
		sem[i] = &KeyedSemaphore[K]{
			maxSize: maxSize,
			semMap:  make(map[K]chan struct{}),
		}
	}
	return sem
}

func (sks ShardedKeyedSemaphore[K]) GetShard(key K) *KeyedSemaphore[K] {
	shardIndex := int(hash(key)) % len(sks)
	return sks[shardIndex]
}

func hash[K comparable](key K) uint32 {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(key)
	if err != nil {
		log.Fatalf("failed to encode key: %v", err)
	}
	h := fnv.New32a()
	h.Write(buf.Bytes())
	return h.Sum32()
}

func NewKeyedSemaphore[K comparable](maxSize int) *KeyedSemaphore[K] {
	return &KeyedSemaphore[K]{
		maxSize: maxSize,
		semMap:  make(map[K]chan struct{}),
	}
}

func (ks *KeyedSemaphore[K]) Wait(ctx context.Context, key K) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	ks.mu.RLock()
	sem, exists := ks.semMap[key]
	ks.mu.RUnlock()

	if !exists {
		ks.mu.Lock()
		sem, exists = ks.semMap[key]
		if !exists {
			select {
			case <-ctx.Done():
				ks.mu.Unlock()
				return ctx.Err()
			default:
			}
			sem = make(chan struct{}, ks.maxSize)
			ks.semMap[key] = sem
		}
		ks.mu.Unlock()
	}

	select {
	case sem <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (ks *KeyedSemaphore[K]) TryWait(ctx context.Context, key K) bool {
	select {
	case <-ctx.Done():
		return false
	default:
	}

	ks.mu.RLock()
	sem, exists := ks.semMap[key]
	ks.mu.RUnlock()

	if !exists {
		ks.mu.Lock()
		sem, exists = ks.semMap[key]
		if !exists {
			sem = make(chan struct{}, ks.maxSize)
			ks.semMap[key] = sem
		}
		ks.mu.Unlock()
	}

	select {
	case sem <- struct{}{}:
		return true
	default:
		return false
	}
}

func (ks *KeyedSemaphore[K]) Release(key K) error {
	ks.mu.Lock()
	sem, exists := ks.semMap[key]
	if !exists {
		ks.mu.Unlock()
		return fmt.Errorf("attempting to release a semaphore that doesn't exist for key: %v", key)
	}

	select {
	case <-sem:
		if len(sem) == 0 {
			delete(ks.semMap, key)
		}
		ks.mu.Unlock()
		return nil
	default:
		ks.mu.Unlock()
		return fmt.Errorf("release called without a matching Wait for key: %v", key)
	}
}
