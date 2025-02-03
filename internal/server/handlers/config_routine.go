package handlers

import (
	"net/http"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
)

// AddRoutine
// @Summary     Adds a backup routine to the config.
// @ID          AddRoutine
// @Tags        Configuration
// @Router      /v1/config/routines/{name} [post]
// @Accept      json
// @Param       name path string true "Backup routine name"
// @Param       routine body dto.BackupRoutine true "Backup routine details"
// @Success     201
// @Failure     400 {string} string
//
//nolint:dupl
func (s *Service) AddRoutine(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		httpError(w, errMissingRoutineName)
		return
	}
	newRoutine, err := dto.NewRoutineFromReader(r.Body, dto.JSON)
	if err != nil {
		httpError(w, errInvalidJSONPayload(err))
		return
	}
	r.Body.Close()
	toModel, err := newRoutine.ToModel(s.config.BackupConfigCopy(), s.nsValidator)
	if err != nil {
		httpError(w, errBadRequest(err))
		return
	}

	err = s.changeConfig(r.Context(), func(config *model.Config) error {
		return config.AddRoutine(name, toModel)
	})
	if err != nil {
		httpError(w, errBadRequest(err))
		return
	}

	w.WriteHeader(http.StatusCreated)
}

// ReadRoutines reads all backup routines from the configuration.
// @Summary     Reads all routines from the configuration.
// @ID	        ReadRoutines
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
// @ID	        ReadRoutine
// @Tags        Configuration
// @Router      /v1/config/routines/{name} [get]
// @Param       name path string true "Backup routine name"
// @Produce     json
// @Success  	200 {object} dto.BackupRoutine
// @Response    400 {string} string
// @Failure     404 {string} string "The specified cluster could not be found"
func (s *Service) ReadRoutine(w http.ResponseWriter, r *http.Request) {
	routineName := r.PathValue("name")
	if routineName == "" {
		httpError(w, errMissingRoutineName)
		return
	}
	routine, ok := s.config.Routines()[routineName]
	if !ok {
		httpError(w, errRoutineNotFound(routineName))
		return
	}

	httpOK(w, dto.NewRoutineFromModel(routine, s.config))
}

// UpdateRoutine updates an existing backup routine in the configuration.
// @Summary      Updates an existing routine in the configuration.
// @ID 	         UpdateRoutine
// @Tags         Configuration
// @Router       /v1/config/routines/{name} [put]
// @Accept       json
// @Param        name path string true "Backup routine name"
// @Param        routine body dto.BackupRoutine true "Backup routine details"
// @Success      200
// @Failure      400 {string} string
//
//nolint:dupl
func (s *Service) UpdateRoutine(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		httpError(w, errMissingRoutineName)
		return
	}

	updatedRoutine, err := dto.NewRoutineFromReader(r.Body, dto.JSON)
	if err != nil {
		httpError(w, errInvalidJSONPayload(err))
		return
	}
	r.Body.Close()

	toModel, err := updatedRoutine.ToModel(s.config.BackupConfigCopy(), s.nsValidator)
	if err != nil {
		httpError(w, errBadRequest(err))
		return
	}

	err = s.changeConfig(r.Context(), func(config *model.Config) error {
		return config.UpdateRoutine(name, toModel)
	})
	if err != nil {
		httpError(w, errBadRequest(err))
		return
	}

	w.WriteHeader(http.StatusOK)
}

// DeleteRoutine
// @Summary     Deletes a backup routine from the configuration by name.
// @ID          DeleteRoutine
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

	err := s.changeConfig(r.Context(), func(config *model.Config) error {
		return config.DeleteRoutine(routineName)
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
// @Failure     404 {string} string
// @Router      /v1/config/routines/{name}/enable [put]
func (s *Service) EnableRoutine(w http.ResponseWriter, r *http.Request) {
	routineName := r.PathValue("name")
	if routineName == "" {
		httpError(w, errMissingRoutineName)
		return
	}
	_, ok := s.handlerHolder.Load(routineName)
	if !ok {
		httpError(w, errRoutineNotFound(routineName))
		return
	}

	err := s.changeConfig(r.Context(), func(config *model.Config) error {
		return config.ToggleRoutineDisabled(routineName, false)
	})
	if err != nil {
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
// @Failure     404 {string} string
// @Failure     500 {string} string "Unexpected error occurred."
// @Router      /v1/config/routines/{name}/disable [put]
func (s *Service) DisableRoutine(w http.ResponseWriter, r *http.Request) {
	routineName := r.PathValue("name")
	if routineName == "" {
		httpError(w, errMissingRoutineName)
		return
	}
	_, found := s.handlerHolder.Load(routineName)
	if !found {
		httpError(w, errRoutineNotFound(routineName))
		return
	}

	err := s.changeConfig(r.Context(), func(config *model.Config) error {
		return config.ToggleRoutineDisabled(routineName, true)
	})
	if err != nil {
		httpError(w, errBadRequest(err))
		return
	}

	s.registry.Cancel(routineName) // cancel any running job for this routine after disabling it.

	w.WriteHeader(http.StatusNoContent)
}
