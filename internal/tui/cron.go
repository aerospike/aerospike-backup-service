package tui

import (
	"strings"

	"github.com/lnquy/cron"
)

func DescribeCron(expr string) string {
	if expr == "" {
		return "never"
	}

	// Normalize shorthands like @daily -> Quartz
	q, err := toQuartz(expr)
	if err != nil {
		return expr
	}

	// Build the descriptor
	desc, err := cron.NewDescriptor(
		cron.Use24HourTimeFormat(true),
		cron.Verbose(false),
	)
	if err != nil {
		return expr
	}

	// Convert to human-friendly English
	s, err := desc.ToDescription(q, cron.Locale_en)
	if err != nil {
		return expr
	}

	return s
}

func toQuartz(in string) (string, error) {
	in = strings.TrimSpace(in)
	if strings.HasPrefix(in, "@") {
		switch strings.ToLower(in) {
		case "@yearly", "@annually":
			return "0 0 0 1 1 ? *", nil
		case "@monthly":
			return "0 0 0 1 * ? *", nil
		case "@weekly":
			return "0 0 0 ? * 1 *", nil
		case "@daily":
			return "0 0 0 * * ? *", nil
		case "@hourly":
			return "0 0 * * * ? *", nil
		default:
			return in, nil
		}
	}

	fields := strings.Fields(in)
	switch len(fields) {
	case 5: // standard cron, prepend seconds + add year wildcard
		return "0 " + in + " *", nil
	case 6: // quartz without year
		return in + " *", nil
	case 7: // already quartz
		return in, nil
	default:
		return in, nil
	}
}
