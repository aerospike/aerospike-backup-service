package cluster

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/aerospike/backup-go/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Test: immediate stability (first call returns 0, no ticker involvement).
func TestWaitForMigrations_NoPendingInitially(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	namespace := "testns"

	ig := mocks.NewMockInfoGetter(t)
	ig.EXPECT().
		GetPendingMigrations(mock.Anything, namespace).
		Return(uint64(0), nil).
		Once()

	err := WaitForMigrations(ctx, ig, namespace)
	assert.NoError(t, err, "expected no error when there are no pending migrations initially")
}

// Test: pending at first, then eventually reaches 0.
func TestWaitForMigrations_PendingThenComplete(t *testing.T) {
	// This test will take a bit over 2 seconds because of the 1s ticker.
	ctx := context.Background()
	namespace := "testns"

	ig := mocks.NewMockInfoGetter(t)

	// Initial immediate call: migrations in progress.
	ig.EXPECT().
		GetPendingMigrations(mock.Anything, namespace).
		Return(uint64(3), nil).
		Once()

	// First ticker tick: still pending.
	ig.EXPECT().
		GetPendingMigrations(mock.Anything, namespace).
		Return(uint64(1), nil).
		Once()

	// Second ticker tick: done.
	ig.EXPECT().
		GetPendingMigrations(mock.Anything, namespace).
		Return(uint64(0), nil).
		Once()

	err := WaitForMigrations(ctx, ig, namespace)
	assert.NoError(t, err, "expected no error when migrations eventually complete")
}

// Test: initial error, then success via ticker polling.
func TestWaitForMigrations_InitialErrorThenComplete(t *testing.T) {
	// Also takes a couple of seconds due to the ticker.
	ctx := context.Background()
	namespace := "testns"

	ig := mocks.NewMockInfoGetter(t)

	// Initial call fails (e.g. cluster unstable).
	ig.EXPECT().
		GetPendingMigrations(mock.Anything, namespace).
		Return(uint64(0), fmt.Errorf("transient error")).
		Once()

	// First ticker tick: migrations are in progress.
	ig.EXPECT().
		GetPendingMigrations(mock.Anything, namespace).
		Return(uint64(2), nil).
		Once()

	// Second ticker tick: done.
	ig.EXPECT().
		GetPendingMigrations(mock.Anything, namespace).
		Return(uint64(0), nil).
		Once()

	err := WaitForMigrations(ctx, ig, namespace)
	assert.NoError(t, err, "expected no error even if initial call fails but later succeeds")
}

// Test: context is already cancelled before we call WaitForMigrations.
// We still do the initial call, then immediately exit on ctx.Done() in the loop.
func TestWaitForMigrations_ContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before calling the function

	namespace := "testns"

	ig := mocks.NewMockInfoGetter(t)

	// We still expect the initial call to happen (we pass a ctx that’s already done,
	// but the mock doesn't care about the ctx state).
	ig.EXPECT().
		GetPendingMigrations(mock.Anything, namespace).
		Return(uint64(5), nil).
		Once()

	err := WaitForMigrations(ctx, ig, namespace)
	if assert.Error(t, err, "expected error when context is canceled") {
		assert.True(t, errors.Is(err, context.Canceled), "error should wrap context.Canceled")
	}
}
