package keyedsemaphore

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestKeyedSemaphore_Wait(t *testing.T) {
	tests := []struct {
		name    string
		maxSize int
		key     string
		ctx     context.Context
		wantErr bool
	}{
		{
			name:    "Normal case",
			maxSize: 1,
			key:     "key1",
			ctx:     context.Background(),
			wantErr: false,
		},
		{
			name:    "Context cancelled",
			maxSize: 1,
			key:     "key2",
			ctx: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			}(),
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ks := NewKeyedSemaphore[string](tt.maxSize)
			gotErr := ks.Wait(tt.ctx, tt.key)
			if gotErr != nil {
				if !tt.wantErr {
					t.Errorf("Wait() failed: %v", gotErr)
				}
				return
			}
			if tt.wantErr {
				t.Fatal("Wait() succeeded unexpectedly")
			}
		})
	}
}

func TestKeyedSemaphore_WaitReleaseInteraction(t *testing.T) {
	t.Run("Wait blocks when full and unblocks on Release", func(t *testing.T) {
		ks := NewKeyedSemaphore[string](1)
		key := "blockKey"
		ctx := context.Background()

		// First Wait should succeed
		err := ks.Wait(ctx, key)
		require.NoError(t, err, "First Wait failed unexpectedly")

		// Start second Wait in a goroutine, it should block
		waitChan := make(chan error, 1)
		go func() {
			waitChan <- ks.Wait(ctx, key)
		}()

		// Give the goroutine a chance to start and potentially block
		// A select ensures we don't wait unnecessarily long if it doesn't block.
		select {
		case err := <-waitChan:
			t.Fatalf("Second Wait completed unexpectedly before Release: %v", err)
		case <-time.After(50 * time.Millisecond):
			// Expected: It's likely blocked, proceed
		}

		// Release the key
		err = ks.Release(key)
		require.NoError(t, err, "Release failed unexpectedly")

		// Now the second Wait should succeed
		select {
		case err := <-waitChan:
			require.NoError(t, err, "Second Wait failed after Release")
		case <-time.After(100 * time.Millisecond): // Timeout
			t.Fatal("Second Wait did not complete after Release")
		}
	})
}
