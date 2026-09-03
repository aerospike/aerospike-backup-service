package timeutil

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMeasureDuration(t *testing.T) {
	duration, err := MeasureDuration(func() error {
		time.Sleep(10 * time.Millisecond)
		return nil
	})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, duration, 10*time.Millisecond)

	expectedErr := errors.New("boom")
	duration, err = MeasureDuration(func() error {
		return expectedErr
	})
	require.ErrorIs(t, err, expectedErr)
	assert.GreaterOrEqual(t, duration, time.Duration(0))
}

func TestMeasureDurationWithResult(t *testing.T) {
	value, duration, err := MeasureDurationWithResult(func() (string, error) {
		time.Sleep(10 * time.Millisecond)
		return "ok", nil
	})
	require.NoError(t, err)
	assert.Equal(t, "ok", value)
	assert.GreaterOrEqual(t, duration, 10*time.Millisecond)

	expectedErr := errors.New("fail")
	intValue, duration, err := MeasureDurationWithResult(func() (int, error) {
		return 0, expectedErr
	})
	require.ErrorIs(t, err, expectedErr)
	assert.Equal(t, 0, intValue)
	assert.GreaterOrEqual(t, duration, time.Duration(0))
}
