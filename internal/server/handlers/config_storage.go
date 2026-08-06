package handlers

import (
	"net/http"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto/decoder"
)

// AddStorage
// @Summary     Adds a storage to the config.
// @ID	        addStorage
// @Tags        Configuration
// @Router      /v1/config/storage/{name} [post]
// @Accept      json
// @Param       name path string true "Backup storage name"
// @Param       storage body dto.Storage true "Backup storage details"
// @Success     201
// @Failure     400 {string} string
func (s *Service) AddStorage(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		httpError(w, errMissingStorageName)
		return
	}

	newStorage, err := dto.NewStorageFromReader(r.Body, decoder.JSON)
	if err != nil {
		httpError(w, errInvalidJSONPayload(err))
		return
	}

	if err = s.configManager.AddStorage(r.Context(), name, newStorage); err != nil {
		httpError(w, errBadRequest(err))
		return
	}

	w.WriteHeader(http.StatusCreated)
}

// ReadAllStorage reads all storage from the configuration.
// @Summary     Reads all storage from the configuration.
// @ID 	        readAllStorage
// @Tags        Configuration
// @Router      /v1/config/storage [get]
// @Produce     json
// @Success  	200 {object} map[string]dto.Storage
func (s *Service) ReadAllStorage(w http.ResponseWriter, r *http.Request) {
	httpOK(w, s.configManager.ReadAllStorage(r.Context()))
}

// ReadStorage  reads a specific storage from the configuration given its name.
// @Summary     Reads a specific storage from the configuration given its name.
// @ID	        readStorage
// @Tags        Configuration
// @Router      /v1/config/storage/{name} [get]
// @Param       name path string true "Backup storage name"
// @Produce     json
// @Success  	200 {object} dto.Storage
// @Response    400 {string} string
// @Failure     404 {string} string "The specified storage was not found"
func (s *Service) ReadStorage(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		httpError(w, errMissingStorageName)
		return
	}

	storage, err := s.configManager.ReadStorage(r.Context(), name)
	if err != nil {
		httpError(w, errNotFound("storage", name))
		return
	}

	httpOK(w, storage)
}

// UpdateStorage updates an existing storage in the configuration.
// @Summary     Updates an existing storage in the configuration.
// @ID	        updateStorage
// @Tags        Configuration
// @Router      /v1/config/storage/{name} [put]
// @Accept      json
// @Param       name path string true "Backup storage name"
// @Param       storage body dto.Storage true "Backup storage details"
// @Success     200
// @Failure     400 {string} string
func (s *Service) UpdateStorage(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		httpError(w, errMissingStorageName)
		return
	}

	updatedStorage, err := dto.NewStorageFromReader(r.Body, decoder.JSON)
	if err != nil {
		httpError(w, errInvalidJSONPayload(err))
		return
	}

	if err = s.configManager.UpdateStorage(r.Context(), name, updatedStorage); err != nil {
		httpError(w, errBadRequest(err))
		return
	}

	w.WriteHeader(http.StatusOK)
}

// DeleteStorage
// @Summary     Deletes a storage from the configuration by name.
// @ID	        deleteStorage
// @Tags        Configuration
// @Router      /v1/config/storage/{name} [delete]
// @Param       name path string true "Backup storage name"
// @Success     204
// @Failure     400 {string} string
func (s *Service) DeleteStorage(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		httpError(w, errMissingStorageName)
		return
	}

	err := s.configManager.DeleteStorage(r.Context(), name)
	if err != nil {
		httpError(w, errBadRequest(err))
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
