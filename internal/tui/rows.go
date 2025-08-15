package tui

import (
	m "github.com/aerospike/aerospike-backup-service/v3/pkg/model"
)

type rowKind int

const (
	rowPlus rowKind = iota
	rowRunning
	rowFull
	rowIncr
	rowSep
)

type row struct {
	kind   rowKind
	label  string           // для + и разводок
	backup *m.BackupDetails // для running / full / incr
}

func groupRows(in []m.BackupDetails) []row {
	var out []row
	for i := 0; i < len(in); {
		b := in[i]
		if b.IsFull() {
			out = append(out, row{kind: rowFull, backup: &b})
			j := i + 1
			for j < len(in) && !(in[j]).IsFull() {
				bj := in[j]
				out = append(out, row{kind: rowIncr, backup: &bj})
				j++
			}
			i = j
		} else {
			out = append(out, row{kind: rowIncr, backup: &b})
			i++
		}
	}
	return out
}
