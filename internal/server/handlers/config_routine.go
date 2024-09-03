package handlers

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/aerospike/aerospike-backup-service/v2/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/v2/pkg/model"
	"github.com/gorilla/mux"
)

const routineNameNotSpecifiedMsg = "Routine name is not specified"

func (s *Service) ConfigRoutineActionHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		s.addRoutine(w, r)
	case http.MethodGet:
		s.readRoutine(w, r)
	case http.MethodPut:
		s.updateRoutine(w, r)
	case http.MethodDelete:
		s.deleteRoutine(w, r)
	}
}

// addRoutine
// @Summary     Adds a backup routine to the config.
// @ID          addRoutine
// @Tags        Configuration
// @Router      /v1/config/routines/{name} [post]
// @Accept      json
// @Param       name path string true "Backup routine name"
// @Param       routine body dto.BackupRoutine true "Backup routine details"
// @Success     201
// @Failure     400 {string} string
//
//nolint:dupl
func (s *Service) addRoutine(w http.ResponseWriter, r *http.Request) {
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
	name := mux.Vars(r)["name"]
	if name == "" {
		hLogger.Error("routine name required")
		http.Error(w, routineNameNotSpecifiedMsg, http.StatusBadRequest)
		return
	}
	toModel, err := newRoutine.ToModel(s.config)
	if err != nil {
		hLogger.Error("failed to create routine",
			slog.String("name", name),
			slog.Any("error", err),
		)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	err = s.config.AddRoutine(name, toModel)
	if err != nil {
		hLogger.Error("failed to add routine",
			slog.String("name", name),
			slog.Any("error", err),
		)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	err = s.configurationManager.WriteConfiguration(s.config)
	if err != nil {
		hLogger.Error("failed to write configuration",
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

	toDTO := dto.ConvertModelMapToDTO(s.config.BackupRoutines, func(m *model.BackupRoutine) *dto.BackupRoutine {
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

// readRoutine reads a specific routine from the configuration given its name.
// @Summary     Reads a specific routine from the configuration given its name.
// @ID	        readRoutine
// @Tags        Configuration
// @Router      /v1/config/routines/{name} [get]
// @Param       name path string true "Backup routine name"
// @Produce     json
// @Success  	200 {object} dto.BackupRoutine
// @Response    400 {string} string
// @Failure     404 {string} string "The specified cluster could not be found"
func (s *Service) readRoutine(w http.ResponseWriter, r *http.Request) {
	hLogger := s.logger.With(slog.String("handler", "readRoutine"))

	routineName := mux.Vars(r)["name"]
	if routineName == "" {
		hLogger.Error("routine name required")
		http.Error(w, routineNameNotSpecifiedMsg, http.StatusBadRequest)
		return
	}
	routine, ok := s.config.BackupRoutines[routineName]
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

// updateRoutine updates an existing backup routine in the configuration.
// @Summary      Updates an existing routine in the configuration.
// @ID 	         updateRoutine
// @Tags         Configuration
// @Router       /v1/config/routines/{name} [put]
// @Accept       json
// @Param        name path string true "Backup routine name"
// @Param        routine body dto.BackupRoutine true "Backup routine details"
// @Success      200
// @Failure      400 {string} string
//
//nolint:dupl
func (s *Service) updateRoutine(w http.ResponseWriter, r *http.Request) {
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
	name := mux.Vars(r)["name"]
	if name == "" {
		hLogger.Error("routine name required")
		http.Error(w, routineNameNotSpecifiedMsg, http.StatusBadRequest)
		return
	}

	toModel, err := updatedRoutine.ToModel(s.config)
	if err != nil {
		hLogger.Error("failed to create routine",
			slog.String("name", name),
			slog.Any("error", err),
		)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	err = s.config.UpdateRoutine(name, toModel)
	if err != nil {
		hLogger.Error("failed to update routine",
			slog.String("name", name),
			slog.Any("error", err),
		)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	err = s.configurationManager.WriteConfiguration(s.config)
	if err != nil {
		hLogger.Error("failed to write configuration",
			slog.String("name", name),
			slog.Any("error", err),
		)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// deleteRoutine
// @Summary     Deletes a backup routine from the configuration by name.
// @ID          deleteRoutine
// @Tags        Configuration
// @Router      /v1/config/routines/{name} [delete]
// @Param       name path string true "Backup routine name"
// @Success     204
// @Failure     400 {string} string
//
//nolint:dupl // Each handler must be in separate func. No duplication.
func (s *Service) deleteRoutine(w http.ResponseWriter, r *http.Request) {
	hLogger := s.logger.With(slog.String("handler", "deleteRoutine"))

	routineName := mux.Vars(r)["name"]
	if routineName == "" {
		hLogger.Error("routine name required")
		http.Error(w, routineNameNotSpecifiedMsg, http.StatusBadRequest)
		return
	}
	err := s.config.DeleteRoutine(routineName)
	if err != nil {
		hLogger.Error("failed to delete routine",
			slog.String("name", routineName),
			slog.Any("error", err),
		)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	err = s.configurationManager.WriteConfiguration(s.config)
	if err != nil {
		hLogger.Error("failed to write configuration",
			slog.String("name", routineName),
			slog.Any("error", err),
		)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
