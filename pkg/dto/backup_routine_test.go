package dto

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidSinglePartitionID(t *testing.T) {
	err := validatePartitionList("0,100,4095")
	assert.NoError(t, err)
}

func TestInvalidPartitionID_OutOfRange(t *testing.T) {
	err := validatePartitionList("4096")
	assert.Error(t, err)
}

func TestValidPartitionRange(t *testing.T) {
	err := validatePartitionList("0-1,100-50,4095-1")
	assert.NoError(t, err)
}

func TestInvalidPartitionRange_StartTooHigh(t *testing.T) {
	err := validatePartitionList("4095-2")
	assert.Error(t, err)
}

func TestInvalidPartitionRange_CountZero(t *testing.T) {
	err := validatePartitionList("100-0")
	assert.Error(t, err)
}

func TestInvalidPartitionRange_BadFormat(t *testing.T) {
	err := validatePartitionList("100--200")
	assert.Error(t, err)
}

func TestEmptyString(t *testing.T) {
	err := validatePartitionList("")
	assert.NoError(t, err)
}

func TestEmptyEntry(t *testing.T) {
	err := validatePartitionList("100,,200")
	assert.Error(t, err)
}
