package handlers

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/aerospike/aerospike-backup-service/v2/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/v2/pkg/model"
	"github.com/gorilla/mux"
)

const storageNameNotSpecifiedMsg = "Storage name is not specified"

func (s *Service) ConfigStorageActionHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		s.addStorage(w, r)
	case http.MethodGet:
		s.readStorage(w, r)
	case http.MethodPut:
		s.updateStorage(w, r)
	case http.MethodDelete:
		s.deleteStorage(w, r)
	}
}

// addStorage
// @Summary     Adds a storage to the config.
// @ID	        addStorage
// @Tags        Configuration
// @Router      /v1/config/storage/{name} [post]
// @Accept      json
// @Param       name path string true "Backup storage name"
// @Param       storage body dto.Storage true "Backup storage details"
// @Success     201
// @Failure     400 {string} string
//
//nolint:dupl
func (s *Service) addStorage(w http.ResponseWriter, r *http.Request) {
	hLogger := s.logger.With(slog.String("handler", "addStorage"))

	newStorage, err := dto.NewStorageFromReader(r.Body, dto.JSON)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	r.Body.Close()
	name := mux.Vars(r)["name"]
	if name == "" {
		hLogger.Error("storage name required")
		http.Error(w, storageNameNotSpecifiedMsg, http.StatusBadRequest)
		return
	}

	err = s.changeConfig(r.Context(), func(config *model.Config) error {
		return config.AddStorage(name, newStorage.ToModel())
	})
	if err != nil {
		hLogger.Error("failed to add storage",
			slog.String("name", name),
			slog.Any("error", err),
		)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusCreated)
}

// ReadAllStorage reads all storage from the configuration.
// @Summary     Reads all storage from the configuration.
// @ID 	        ReadAllStorage
// @Tags        Configuration
// @Router      /v1/config/storage [get]
// @Produce     json
// @Success  	200 {object} map[string]dto.Storage
// @Failure     500 {string} string
func (s *Service) ReadAllStorage(w http.ResponseWriter, _ *http.Request) {
	hLogger := s.logger.With(slog.String("handler", "ReadAllStorage"))

	toDTO := dto.ConvertStorageMapToDTO(s.config.Storage)
	jsonResponse, err := dto.Serialize(toDTO, dto.JSON)
	if err != nil {
		hLogger.Error("failed to marshal storage",
			slog.Any("error", err),
		)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, err = w.Write(jsonResponse)
	if err != nil {
		hLogger.Error("failed to write response",
			slog.String("response", string(jsonResponse)),
			slog.Any("error", err),
		)
	}
}

// readStorage  reads a specific storage from the configuration given its name.
// @Summary     Reads a specific storage from the configuration given its name.
// @ID	        readStorage
// @Tags        Configuration
// @Router      /v1/config/storage/{name} [get]
// @Param       name path string true "Backup storage name"
// @Produce     json
// @Success  	200 {object} dto.Storage
// @Response    400 {string} string
// @Failure     404 {string} string "The specified storage could not be found"
// @Failure     500 {string} string
func (s *Service) readStorage(w http.ResponseWriter, r *http.Request) {
	hLogger := s.logger.With(slog.String("handler", "readStorage"))

	storageName := mux.Vars(r)["name"]
	if storageName == "" {
		hLogger.Error("storage name required")
		http.Error(w, storageNameNotSpecifiedMsg, http.StatusBadRequest)
		return
	}
	storage, ok := s.config.Storage[storageName]
	if !ok {
		http.Error(w, fmt.Sprintf("Storage %s could not be found", storageName), http.StatusNotFound)
		return
	}

	jsonResponse, err := dto.Serialize(dto.NewStorageFromModel(storage), dto.JSON)
	if err != nil {
		hLogger.Error("failed to marshal storage",
			slog.Any("error", err),
		)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, err = w.Write(jsonResponse)
	if err != nil {
		hLogger.Error("failed to write response",
			slog.String("response", string(jsonResponse)),
			slog.Any("error", err),
		)
	}
}

// updateStorage updates an existing storage in the configuration.
// @Summary     Updates an existing storage in the configuration.
// @ID	        updateStorage
// @Tags        Configuration
// @Router      /v1/config/storage/{name} [put]
// @Accept      json
// @Param       name path string true "Backup storage name"
// @Param       storage body dto.Storage true "Backup storage details"
// @Success     200
// @Failure     400 {string} string
func (s *Service) updateStorage(w http.ResponseWriter, r *http.Request) {
	hLogger := s.logger.With(slog.String("handler", "updateStorage"))

	updatedStorage, err := dto.NewStorageFromReader(r.Body, dto.JSON)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	storageName := mux.Vars(r)["name"]
	if storageName == "" {
		hLogger.Error("storage name required")
		http.Error(w, storageNameNotSpecifiedMsg, http.StatusBadRequest)
		return
	}

	err = s.changeConfig(r.Context(), func(config *model.Config) error {
		return config.UpdateStorage(storageName, updatedStorage.ToModel())
	})
	if err != nil {
		hLogger.Error("failed to update storage",
			slog.String("name", storageName),
			slog.Any("error", err),
		)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// deleteStorage
// @Summary     Deletes a storage from the configuration by name.
// @ID	        deleteStorage
// @Tags        Configuration
// @Router      /v1/config/storage/{name} [delete]
// @Param       name path string true "Backup storage name"
// @Success     204
// @Failure     400 {string} string
func (s *Service) deleteStorage(w http.ResponseWriter, r *http.Request) {
	hLogger := s.logger.With(slog.String("handler", "deleteStorage"))

	storageName := mux.Vars(r)["name"]
	if storageName == "" {
		hLogger.Error("storage name required")
		http.Error(w, storageNameNotSpecifiedMsg, http.StatusBadRequest)
		return
	}

	err := s.changeConfig(r.Context(), func(config *model.Config) error {
		return config.DeleteStorage(storageName)
	})
	if err != nil {
		hLogger.Error("failed to delete storage",
			slog.String("name", storageName),
			slog.Any("error", err),
		)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
