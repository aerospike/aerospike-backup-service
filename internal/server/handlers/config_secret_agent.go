package handlers

import (
	"fmt"
	"net/http"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto/decoder"
)

// CheckSecretAgentConnectivity
// @Summary     Checks connectivity to a Secret Agent.
// @ID          checkSecretAgentConnectivity
// @Tags        Configuration
// @Router      /v1/config/secret-agents/check-connectivity [post]
// @Accept      json
// @Param       secretAgent body dto.SecretAgent true "Secret Agent details"
// @Success     200 {string} string "Connection successful"
// @Failure     400 {string} string
// @Failure     500 {string} string
func (s *Service) CheckSecretAgentConnectivity(w http.ResponseWriter, r *http.Request) {
	newAgent, err := dto.NewSecretAgentFromReader(r.Body, decoder.JSON)
	if err != nil {
		httpError(w, errInvalidJSONPayload(err))
		return
	}

	agentModel := newAgent.ToModel()

	if err := agentModel.CheckSecretAgentConnection(); err != nil {
		httpError(w, errBadRequest(fmt.Errorf("failed to connect to secret agent: %w", err)))
		return
	}

	httpOK(w, "Connection successful")
}
