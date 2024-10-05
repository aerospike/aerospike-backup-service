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

func newNameValidator(s string) *nameValidator {
	if s != "" {
		return &nameValidator{s}
	}

	return nil
}

func (n *nameValidator) Run(path string) error {
	if n == nil || len(n.filter) == 0 {
		return nil
	}

	if strings.HasSuffix(path, n.filter) {
		return nil
	}

	return fmt.Errorf("skipped by filter '%s'", n.filter)
}
