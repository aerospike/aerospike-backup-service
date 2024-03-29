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
				AuthMode:     nil,
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
				AuthMode:     nil,
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
			os.RemoveAll(testdataFolder)
		})
	}
}

func TestAerospikeCluster_GetPasswordCaching(t *testing.T) {
	createValidFile()

	cluster := &AerospikeCluster{
		Credentials: &Credentials{
			User:         nil,
			PasswordPath: ptr.String(passwordPath),
			AuthMode:     nil,
		},
	}

	password := cluster.GetPassword()
	assert.Equal(t, ptr.String("password"), password)

	// remove file to ensure second call will not read it
	os.RemoveAll(testdataFolder)

	// Make a second call to GetPassword and check if the returned passwords are same
	passwordAfterCache := cluster.GetPassword()
	assert.Equal(t, password, passwordAfterCache)
}

func createValidFile() {
	text := []byte("password")
	os.MkdirAll(testdataFolder, 0744)
	f, _ := os.OpenFile(passwordPath, os.O_WRONLY|os.O_CREATE, 0644)
	defer f.Close()
	f.Write(text)
}
