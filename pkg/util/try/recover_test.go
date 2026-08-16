package try

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRecover_NormalReturn(t *testing.T) {
	value, err := Recover(func() string {
		return "ok"
	})
	require.NoError(t, err)
	assert.Equal(t, "ok", value)
}

func TestRecover_Panic(t *testing.T) {
	value, err := Recover(func() string {
		panic("unexpected")
	})
	assert.Empty(t, value)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "recovered from panic")
}

func TestRecoverError_NormalReturn(t *testing.T) {
	value, err := RecoverError(func() (int, error) {
		return 42, nil
	})
	require.NoError(t, err)
	assert.Equal(t, 42, value)
}

func TestRecoverError_Panic(t *testing.T) {
	value, err := RecoverError(func() (int, error) {
		panic("boom")
	})
	assert.Equal(t, 0, value)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "recovered from panic")
}
