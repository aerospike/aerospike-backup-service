package dto

import (
	"strings"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
)

func NewStatusFilterFromString(statusParam string) (model.StatusFilter, error) {
	fields := make([]model.JobStatus, 0)

	isExclude := strings.HasPrefix(statusParam, "!")
	if isExclude {
		statusParam = statusParam[1:]
	}

	for _, s := range strings.Split(statusParam, ",") {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}

		status, ok := RestoreStatusFromString(s)
		if !ok {
			return model.StatusFilter{}, errValidationInvalidValue("status", s, allJobStatuses)
		}
		fields = append(fields, model.JobStatus(status))
	}

	return model.NewStatusFilter(fields, isExclude), nil
}
