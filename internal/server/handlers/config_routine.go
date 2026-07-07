package handlers

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto/decoder"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
)

// AddRoutine
// @Summary     Adds a backup routine to the config.
// @ID          addRoutine
// @Tags        Configuration
// @Router      /v1/config/routines/{name} [post]
// @Accept      json
// @Param       name path string true "Backup routine name"
// @Param       routine body dto.BackupRoutine true "Backup routine details"
// @Success     201
// @Failure     400 {string} string
func (s *Service) AddRoutine(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		httpError(w, errMissingRoutineName)
		return
	}
	newRoutine, err := dto.NewRoutineFromReader(r.Body, decoder.JSON)
	if err != nil {
		httpError(w, errInvalidJSONPayload(err))
		return
	}

	if err = s.changeBackupConfig(r.Context(), func(config *dto.Config) ([]string, error) {
		if _, exists := config.BackupRoutines[name]; exists {
			return nil, fmt.Errorf("add backup routine %q: %w", name, model.ErrAlreadyExists)
		}
		config.BackupRoutines[name] = newRoutine
		return []string{name}, nil
	}, withNamespaceValidation); err != nil {
		httpError(w, errBadRequest(err))
		return
	}

	w.WriteHeader(http.StatusCreated)
}

// ReadRoutines reads all backup routines from the configuration.
// @Summary     Reads all routines from the configuration.
// @ID	        readRoutines
// @Tags        Configuration
// @Router      /v1/config/routines [get]
// @Produce     json
// @Success  	200 {object} map[string]dto.BackupRoutine
// @Failure     400 {string} string
func (s *Service) ReadRoutines(w http.ResponseWriter, _ *http.Request) {
	routines := dto.ConvertModelMapToDTO(s.config.Routines(), func(m *model.BackupRoutine) *dto.BackupRoutine {
		return dto.NewRoutineFromModel(m, s.config)
	})

	httpOK(w, routines)
}

// ReadRoutine reads a specific routine from the configuration given its name.
// @Summary     Reads a specific routine from the configuration given its name.
// @ID	        readRoutine
// @Tags        Configuration
// @Router      /v1/config/routines/{name} [get]
// @Param       name path string true "Backup routine name"
// @Produce     json
// @Success  	200 {object} dto.BackupRoutine
// @Response    400 {string} string
// @Failure     404 {string} string "The specified routine was not found"
func (s *Service) ReadRoutine(w http.ResponseWriter, r *http.Request) {
	routineName := r.PathValue("name")
	if routineName == "" {
		httpError(w, errMissingRoutineName)
		return
	}
	routine, found := s.config.Routine(routineName)
	if !found {
		httpError(w, errRoutineNotFound(routineName))
		return
	}

	httpOK(w, dto.NewRoutineFromModel(routine, s.config))
}

// UpdateRoutine updates an existing backup routine in the configuration.
// @Summary      Updates an existing routine in the configuration.
// @ID 	         updateRoutine
// @Tags         Configuration
// @Router       /v1/config/routines/{name} [put]
// @Accept       json
// @Param        name path string true "Backup routine name"
// @Param        routine body dto.BackupRoutine true "Backup routine details"
// @Success      200
// @Failure      400 {string} string
func (s *Service) UpdateRoutine(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		httpError(w, errMissingRoutineName)
		return
	}

	updatedRoutine, err := dto.NewRoutineFromReader(r.Body, decoder.JSON)
	if err != nil {
		httpError(w, errInvalidJSONPayload(err))
		return
	}

	if err = s.changeBackupConfig(r.Context(), func(config *dto.Config) ([]string, error) {
		if _, exists := config.BackupRoutines[name]; !exists {
			return nil, fmt.Errorf("update backup routine %q: %w", name, model.ErrNotFound)
		}
		config.BackupRoutines[name] = updatedRoutine
		return []string{name}, nil
	}, withNamespaceValidation); err != nil {
		httpError(w, errBadRequest(err))
		return
	}

	w.WriteHeader(http.StatusOK)
}

// DeleteRoutine
// @Summary     Deletes a backup routine from the configuration by name.
// @ID          deleteRoutine
// @Tags        Configuration
// @Router      /v1/config/routines/{name} [delete]
// @Param       name path string true "Backup routine name"
// @Success     204
// @Failure     400 {string} string
func (s *Service) DeleteRoutine(w http.ResponseWriter, r *http.Request) {
	routineName := r.PathValue("name")
	if routineName == "" {
		httpError(w, errMissingRoutineName)
		return
	}

	err := s.changeBackupConfig(r.Context(), func(config *dto.Config) ([]string, error) {
		if _, exists := config.BackupRoutines[routineName]; !exists {
			return nil, fmt.Errorf("delete backup routine %q: %w", routineName, model.ErrNotFound)
		}
		delete(config.BackupRoutines, routineName)
		return nil, nil
	})
	if err != nil {
		httpError(w, errBadRequest(err))
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// EnableRoutine
// @Summary     Enable a backup routine.
// @Tags        Configuration
// @ID          enableRoutine
// @Param       name path string true "Backup routine name"
// @Success     204 "Routine successfully enabled."
// @Failure     404 {string} string "The specified routine was not found"
// @Router      /v1/config/routines/{name}/enable [put]
func (s *Service) EnableRoutine(w http.ResponseWriter, r *http.Request) {
	routineName := r.PathValue("name")
	if routineName == "" {
		httpError(w, errMissingRoutineName)
		return
	}

	err := s.changeBackupConfig(r.Context(), func(config *dto.Config) ([]string, error) {
		routine, exists := config.BackupRoutines[routineName]
		if !exists {
			return nil, fmt.Errorf("toggle disable for backup routine %q: %w", routineName, model.ErrNotFound)
		}
		routine.Disabled = false
		return []string{routineName}, nil
	})
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			httpError(w, errRoutineNotFound(routineName))
			return
		}
		httpError(w, errBadRequest(err))
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// DisableRoutine
// @Summary     Disable a backup routine.
// @Tags        Configuration
// @ID          disableRoutine
// @Param       name path string true "The name of the backup routine."
// @Success     204 "Routine successfully disabled."
// @Failure     404 {string} string "The specified routine was not found"
// @Router      /v1/config/routines/{name}/disable [put]
func (s *Service) DisableRoutine(w http.ResponseWriter, r *http.Request) {
	routineName := r.PathValue("name")
	if routineName == "" {
		httpError(w, errMissingRoutineName)
		return
	}

	err := s.changeBackupConfig(r.Context(), func(config *dto.Config) ([]string, error) {
		routine, exists := config.BackupRoutines[routineName]
		if !exists {
			return nil, fmt.Errorf("toggle disable for backup routine %q: %w", routineName, model.ErrNotFound)
		}
		routine.Disabled = true
		return nil, nil
	}) // name -> routineName
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			httpError(w, errRoutineNotFound(routineName))
			return
		}
		httpError(w, errBadRequest(err))
		return
	}

	s.registry.Cancel(routineName) // cancel any running job for this routine after disabling it.

	w.WriteHeader(http.StatusNoContent)
}
