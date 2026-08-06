package handlers

import (
	"errors"
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

	if err = s.configManager.AddRoutine(r.Context(), name, newRoutine); err != nil {
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
func (s *Service) ReadRoutines(w http.ResponseWriter, r *http.Request) {
	httpOK(w, s.configManager.ReadRoutines(r.Context()))
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
	name := r.PathValue("name")
	if name == "" {
		httpError(w, errMissingRoutineName)
		return
	}

	routine, err := s.configManager.ReadRoutine(r.Context(), name)
	if err != nil {
		httpError(w, errRoutineNotFound(name))
		return
	}

	httpOK(w, routine)
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

	if err = s.configManager.UpdateRoutine(r.Context(), name, updatedRoutine); err != nil {
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
	name := r.PathValue("name")
	if name == "" {
		httpError(w, errMissingRoutineName)
		return
	}

	err := s.configManager.DeleteRoutine(r.Context(), name)
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
	name := r.PathValue("name")
	if name == "" {
		httpError(w, errMissingRoutineName)
		return
	}

	err := s.configManager.EnableRoutine(r.Context(), name)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			httpError(w, errRoutineNotFound(name))
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
	name := r.PathValue("name")
	if name == "" {
		httpError(w, errMissingRoutineName)
		return
	}

	err := s.configManager.DisableRoutine(r.Context(), name)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			httpError(w, errRoutineNotFound(name))
			return
		}
		httpError(w, errBadRequest(err))
		return
	}

	s.registry.Cancel(name) // cancel any running job for this routine after disabling it.

	w.WriteHeader(http.StatusNoContent)
}
