package handlers

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
)

const routineNameNotSpecifiedMsg = "Routine name is not specified"

func (s *Service) ConfigRoutineActionHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		s.AddRoutine(w, r)
	case http.MethodGet:
		s.ReadRoutine(w, r)
	case http.MethodPut:
		s.UpdateRoutine(w, r)
	case http.MethodDelete:
		s.DeleteRoutine(w, r)
	}
}

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
	hLogger := s.logger.With(slog.String("handler", "addRoutine"))

	newRoutine, err := dto.NewRoutineFromReader(r.Body, dto.JSON)
	if err != nil {
		hLogger.Error("failed to decode request body",
			slog.Any("error", err),
		)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	r.Body.Close()
	name := r.PathValue("name")
	if name == "" {
		hLogger.Error("routine name required")
		http.Error(w, routineNameNotSpecifiedMsg, http.StatusBadRequest)
		return
	}
	toModel, err := newRoutine.ToModel(s.config.BackupConfigCopy(), s.nsValidator)
	if err != nil {
		hLogger.Error("failed to create routine",
			slog.String("name", name),
			slog.Any("error", err),
		)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	err = s.changeConfig(r.Context(), func(config *model.Config) error {
		return config.AddRoutine(name, toModel)
	})
	if err != nil {
		hLogger.Error("failed to add routine",
			slog.String("name", name),
			slog.Any("error", err),
		)
		http.Error(w, err.Error(), http.StatusBadRequest)
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
	hLogger := s.logger.With(slog.String("handler", "ReadRoutines"))

	toDTO := dto.ConvertModelMapToDTO(s.config.Routines(), func(m *model.BackupRoutine) *dto.BackupRoutine {
		return dto.NewRoutineFromModel(m, s.config)
	})

	jsonResponse, err := dto.Serialize(toDTO, dto.JSON)
	if err != nil {
		hLogger.Error("failed to marshal backup routines",
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
	hLogger := s.logger.With(slog.String("handler", "readRoutine"))

	routineName := r.PathValue("name")
	if routineName == "" {
		hLogger.Error("routine name required")
		http.Error(w, routineNameNotSpecifiedMsg, http.StatusBadRequest)
		return
	}
	routine, ok := s.config.Routines()[routineName]
	if !ok {
		http.Error(w, fmt.Sprintf("Routine %s could not be found", routineName), http.StatusNotFound)
		return
	}
	jsonResponse, err := dto.Serialize(dto.NewRoutineFromModel(routine, s.config), dto.JSON)
	if err != nil {
		hLogger.Error("failed to marshal backup routines",
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
	hLogger := s.logger.With(slog.String("handler", "updateRoutine"))

	updatedRoutine, err := dto.NewRoutineFromReader(r.Body, dto.JSON)
	if err != nil {
		hLogger.Error("failed to decode request body",
			slog.Any("error", err),
		)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	r.Body.Close()
	name := r.PathValue("name")
	if name == "" {
		hLogger.Error("routine name required")
		http.Error(w, routineNameNotSpecifiedMsg, http.StatusBadRequest)
		return
	}

	toModel, err := updatedRoutine.ToModel(s.config.BackupConfigCopy(), s.nsValidator)
	if err != nil {
		hLogger.Error("failed to create routine",
			slog.String("name", name),
			slog.Any("error", err),
		)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	err = s.changeConfig(r.Context(), func(config *model.Config) error {
		return config.UpdateRoutine(name, toModel)
	})
	if err != nil {
		hLogger.Error("failed to update routine",
			slog.String("name", name),
			slog.Any("error", err),
		)
		http.Error(w, err.Error(), http.StatusBadRequest)
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
	hLogger := s.logger.With(slog.String("handler", "deleteRoutine"))

	routineName := r.PathValue("name")
	if routineName == "" {
		hLogger.Error("routine name required")
		http.Error(w, routineNameNotSpecifiedMsg, http.StatusBadRequest)
		return
	}

	err := s.changeConfig(r.Context(), func(config *model.Config) error {
		return config.DeleteRoutine(routineName)
	})
	if err != nil {
		hLogger.Error("failed to delete routine",
			slog.String("name", routineName),
			slog.Any("error", err),
		)
		http.Error(w, err.Error(), http.StatusBadRequest)
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
	hLogger := s.logger.With(slog.String("handler", "enableRoutine"))

	routineName := r.PathValue("name")
	if routineName == "" {
		hLogger.Error("routine name required")
		http.Error(w, routineNameNotSpecifiedMsg, http.StatusBadRequest)
		return
	}
	_, ok := s.handlerHolder.Load(routineName)
	if !ok {
		hLogger.Error("unknown routine name", slog.String("name", routineName))
		http.Error(w, fmt.Sprintf("Routine %s could not be found", routineName), http.StatusNotFound)
		return
	}

	err := s.changeConfig(r.Context(), func(config *model.Config) error {
		return config.ToggleRoutineDisabled(routineName, false)
	})
	if err != nil {
		hLogger.Error("failed to enable routine",
			slog.String("name", routineName),
			slog.Any("error", err),
		)
		http.Error(w, err.Error(), http.StatusBadRequest)
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
	hLogger := s.logger.With(slog.String("handler", "disableRoutine"))

	routineName := r.PathValue("name")
	if routineName == "" {
		hLogger.Error("routine name required")
		http.Error(w, routineNameNotSpecifiedMsg, http.StatusBadRequest)
		return
	}
	_, found := s.handlerHolder.Load(routineName)
	if !found {
		hLogger.Error("unknown routine name", slog.String("name", routineName))
		http.Error(w, fmt.Sprintf("Routine %s could not be found", routineName), http.StatusNotFound)
		return
	}

	err := s.changeConfig(r.Context(), func(config *model.Config) error {
		return config.ToggleRoutineDisabled(routineName, true)
	})

	s.registry.Cancel(routineName) // cancel any running job for this routine before disabling it.

	if err != nil {
		hLogger.Error("failed to disable routine",
			slog.String("name", routineName),
			slog.Any("error", err),
		)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
