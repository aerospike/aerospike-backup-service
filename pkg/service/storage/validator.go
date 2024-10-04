package storage

import (
	"fmt"
	"strings"
)

type Validator interface {
	Run(fileName string) error
}

type nameValidator struct {
	filter string
}

func (n *nameValidator) Run(path string) error {
	if len(n.filter) == 0 {
		return nil
	}

	if strings.HasSuffix(path, n.filter) {
		return nil
	}

	return fmt.Errorf("skipped by filter '%s'", n.filter)
}
