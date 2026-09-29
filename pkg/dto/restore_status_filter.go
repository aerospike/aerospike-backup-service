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

		status := JobStatus(s)
		if err := status.Validate(); err != nil {
			return model.StatusFilter{}, err
		}
		fields = append(fields, status.ToModel())
	}

	return model.NewStatusFilter(fields, isExclude), nil
}
