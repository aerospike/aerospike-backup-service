package dto

import (
	"strings"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
)

func NewStatusFilterFromString(statusParam string) (model.StatusFilter, error) {
	fields := make([]model.RestoreState, 0)

	isExclude := strings.HasPrefix(statusParam, "!")
	if isExclude {
		statusParam = statusParam[1:]
	}

	for s := range strings.SplitSeq(statusParam, ",") {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}

		rs, ok := model.ParseRestoreState(s)
		if !ok {
			return model.StatusFilter{}, errValidationInvalidValue("status", s, model.AllRestoreStatuses())
		}
		fields = append(fields, rs)
	}

	return model.NewStatusFilter(fields, isExclude), nil
}
