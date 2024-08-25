package model

import (
	"os"
	"testing"

	"github.com/aws/smithy-go/ptr"
	"github.com/stretchr/testify/assert"
)

const (
	testdataFolder = "./testdata"
	passwordPath   = testdataFolder + "/password.txt"
)

func TestAerospikeCluster_GetPassword(t *testing.T) {
	tests := []struct {
		name             string
		setupMock        func()
		credentials      *Credentials
		expectedPassword *string
		expectedErr      bool
	}{
		{
			name:      "ValidPasswordPath",
			setupMock: createValidFile,
			credentials: &Credentials{
				User:         nil,
				PasswordPath: ptr.String(passwordPath),
			},
			expectedPassword: ptr.String("password"),
			expectedErr:      false,
		},
		{
			name:      "InvalidPasswordPath",
			setupMock: func() {},
			credentials: &Credentials{
				User:         nil,
				PasswordPath: ptr.String("not-existing.txt"),
			},
			expectedPassword: nil,
			expectedErr:      true,
		},
		{
			name:             "NoCredentials",
			setupMock:        func() {},
			expectedPassword: nil,
			expectedErr:      true,
		},
		{
			name:             "NoPassword",
			setupMock:        func() {},
			credentials:      &Credentials{},
			expectedPassword: nil,
			expectedErr:      true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.setupMock != nil {
				test.setupMock()
			}
			cluster := &AerospikeCluster{
				Credentials: test.credentials,
			}
			password := cluster.GetPassword()
			assert.Equal(t, test.expectedPassword, password)
			if test.expectedErr {
				assert.Nil(t, password)
			}
			_ = os.RemoveAll(testdataFolder)
		})
	}
}

func TestAerospikeCluster_GetPasswordCaching(t *testing.T) {
	createValidFile()

	cluster := &AerospikeCluster{
		Credentials: &Credentials{
			User:         nil,
			PasswordPath: ptr.String(passwordPath),
		},
	}

	password := cluster.GetPassword()
	assert.Equal(t, ptr.String("password"), password)

	// remove file to ensure second call will not read it
	_ = os.RemoveAll(testdataFolder)

	// Make a second call to GetPassword and check if the returned passwords are same
	passwordAfterCache := cluster.GetPassword()
	assert.Equal(t, password, passwordAfterCache)
}

func TestAerospikeCluster_GetPasswordFromCredentials(t *testing.T) {
	cluster := &AerospikeCluster{
		Credentials: &Credentials{
			User:     nil,
			Password: ptr.String("password"),
		},
	}

	password := cluster.GetPassword()
	assert.Equal(t, ptr.String("password"), password)
}

func createValidFile() {
	text := []byte("password")
	_ = os.MkdirAll(testdataFolder, 0744)
	f, _ := os.OpenFile(passwordPath, os.O_WRONLY|os.O_CREATE, 0644)
	defer func(f *os.File) {
		err := f.Close()
		if err != nil {
			panic(err)
		}
	}(f)
	_, _ = f.Write(text)
}
