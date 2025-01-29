package dto

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidSecretAgent(t *testing.T) {
	port := 8080
	timeout := 5000
	config := SecretAgentConfig{
		SecretAgent: &SecretAgent{
			ConnectionType: "tcp",
			Address:        "localhost",
			Port:           &port,
			Timeout:        &timeout,
		},
	}

	err := config.validate()
	assert.NoError(t, err)
}

func TestValidSecretAgentName(t *testing.T) {
	agentName := "predefined-agent"

	config := SecretAgentConfig{
		SecretAgentName: agentName,
	}

	err := config.validate()
	assert.NoError(t, err)
}

func TestMutuallyExclusive(t *testing.T) {
	agentName := "predefined-agent"
	config := SecretAgentConfig{
		SecretAgent:     &SecretAgent{},
		SecretAgentName: agentName,
	}

	err := config.validate()
	assert.ErrorIs(t, err, errMutuallyExclusive)
}

func TestInvalidSecretAgent(t *testing.T) {
	port := -1 // Invalid port
	config := SecretAgentConfig{
		SecretAgent: &SecretAgent{
			ConnectionType: "tcp",
			Address:        "localhost",
			Port:           &port,
		},
	}

	err := config.validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "secret-agent validation error")
}

func TestSecretAgentConfig_Empty(t *testing.T) {
	config := SecretAgentConfig{}

	err := config.validate()
	assert.NoError(t, err)
}

func TestInvalidConnectionType(t *testing.T) {
	agent := &SecretAgent{
		ConnectionType: "invalid",
		Address:        "localhost",
	}

	err := agent.validate()
	assert.Error(t, err)
	assert.EqualError(t, err, "unsupported connection type: invalid")
}

func TestMissingAddress(t *testing.T) {
	agent := &SecretAgent{
		ConnectionType: "tcp",
	}

	err := agent.validate()
	assert.Error(t, err)
	assert.EqualError(t, err, errValidationEmptyField("address").Error())
}

func TestInvalidTimeout(t *testing.T) {
	timeout := -100
	agent := &SecretAgent{
		ConnectionType: "tcp",
		Address:        "localhost",
		Timeout:        &timeout,
	}

	err := agent.validate()
	assert.Error(t, err)
	assert.EqualError(t, err, errValidationNegative("timeout", -100).Error())
}

func TestSecretAgent_Nil(t *testing.T) {
	var agent *SecretAgent
	err := agent.validate()
	assert.NoError(t, err)
}
