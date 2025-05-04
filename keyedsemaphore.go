package keyedsemaphore

import (
	"context"
	"fmt"
	"sync"
)

type KeyedSemaphore struct {
	maxSize int
	semMap  map[string]chan struct{}
	mu      sync.RWMutex
}

func NewKeyedSemaphore(maxSize int) *KeyedSemaphore {
	return &KeyedSemaphore{
		maxSize: maxSize,
		semMap:  make(map[string]chan struct{}),
	}
}

func (ks *KeyedSemaphore) Wait(key string, ctx context.Context) error {
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

func (ks *KeyedSemaphore) TryWait(ctx context.Context, key string) bool {
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

func (ks *KeyedSemaphore) Release(key string) error {
	ks.mu.Lock()
	sem, exists := ks.semMap[key]
	if !exists {
		ks.mu.Unlock()
		return fmt.Errorf("attempting to release a semaphore that doesn't exist for key: %s", key)
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
		return fmt.Errorf("release called without a matching Wait for key: %s", key)
	}
}
