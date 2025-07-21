package attr

import (
	"log/slog"
)

func Error(value error) slog.Attr    { return slog.Any("error", value) }
func Routine(value string) slog.Attr { return slog.String("routine", value) }
