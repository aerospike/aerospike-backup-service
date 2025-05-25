package dto

import (
	"fmt"
	"strings"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
)

func NewStatusFilterFromString(statusParam string) (model.StatusFilter, error) {
	include := make(map[model.JobStatus]bool)
	exclude := make(map[model.JobStatus]bool)

	if statusParam == "" {
		return model.NewStatusFilter(include, exclude), nil
	}

	for _, s := range strings.Split(statusParam, ",") {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}

		isExclude := strings.HasPrefix(s, "!")
		if isExclude {
			s = s[1:]
		}

		if !isValidStatus(s) {
			return model.StatusFilter{}, fmt.Errorf("invalid status: %s", s)
		}

		if isExclude {
			exclude[model.JobStatus(s)] = true
		} else {
			include[model.JobStatus(s)] = true
		}
	}

	return model.NewStatusFilter(include, exclude), nil
}
