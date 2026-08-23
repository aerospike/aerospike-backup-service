package dto

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidSecretAgent(t *testing.T) {
	port := Port(8080)
	timeout := 5000
	config := SecretAgentConfig{
		SecretAgent: &SecretAgent{
			ConnectionType: "tcp",
			Address:        "localhost",
			Port:           &port,
			Timeout:        &timeout,
		},
	}

	err := config.validate(ValidationDefault)
	require.NoError(t, err)
}

func TestValidSecretAgentName(t *testing.T) {
	agentName := "predefined-agent"

	config := SecretAgentConfig{
		SecretAgentName: agentName,
	}

	err := config.validate(ValidationDefault)
	require.NoError(t, err)
}

func TestMutuallyExclusive(t *testing.T) {
	agentName := "predefined-agent"
	config := SecretAgentConfig{
		SecretAgent:     &SecretAgent{},
		SecretAgentName: agentName,
	}

	err := config.validate(ValidationDefault)
	require.ErrorIs(t, err, errMutuallyExclusive)
}

func TestInvalidSecretAgent(t *testing.T) {
	port := Port(-1) // Invalid port
	config := SecretAgentConfig{
		SecretAgent: &SecretAgent{
			ConnectionType: "tcp",
			Address:        "localhost",
			Port:           &port,
		},
	}

	err := config.validate(ValidationDefault)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "secret-agent validation error")
}

func TestSecretAgentConfig_Empty(t *testing.T) {
	config := SecretAgentConfig{}

	err := config.validate(ValidationDefault)
	require.NoError(t, err)
}

func TestNoConnectionType(t *testing.T) {
	agent := &SecretAgent{
		Address: "localhost",
	}

	err := agent.validate(ValidationDefault)
	require.Error(t, err)
	assert.EqualError(t, err, errValidationEmptyField("connection-type").Error())
}

func TestInvalidConnectionType(t *testing.T) {
	agent := &SecretAgent{
		ConnectionType: "invalid",
		Address:        "localhost",
	}

	err := agent.validate(ValidationDefault)
	require.Error(t, err)
	assert.EqualError(t, err,
		"invalid value validation error: 'invalid' is not a valid connection-type. Allowed values: [tcp unix]")
}

func TestMissingAddress(t *testing.T) {
	agent := &SecretAgent{
		ConnectionType: "tcp",
	}

	err := agent.validate(ValidationDefault)
	require.Error(t, err)
	assert.EqualError(t, err, errValidationEmptyField("address").Error())
}

func TestInvalidTimeout(t *testing.T) {
	timeout := -100
	agent := &SecretAgent{
		ConnectionType: "tcp",
		Address:        "localhost",
		Timeout:        &timeout,
	}

	err := agent.validate(ValidationDefault)
	require.Error(t, err)
	assert.EqualError(t, err, errValidationNegative("timeout", -100).Error())
}

func TestInvalidCertFile(t *testing.T) {
	agent := &SecretAgent{
		Address:        "localhost",
		ConnectionType: "tcp",
		ClientTLS: ClientTLS{
			CAFile: "invalid-cert-file",
		},
	}

	err := agent.validate(ValidationDefault)
	require.Error(t, err)
	require.ErrorContains(t, err, "not found validation error: ca-file \"invalid-cert-file\"")
}

func TestSecretAgent_Nil(t *testing.T) {
	var agent *SecretAgent
	err := agent.validate(ValidationDefault)
	require.NoError(t, err)
}
