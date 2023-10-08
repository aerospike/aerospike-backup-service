package util

import (
	"regexp"
	"strings"

	"log/slog"
)

var libLogRegex = regexp.MustCompile(`^(.+)\s\[(\D+)\]\s\[(\d+)\]\s(.*)$`)

// LogCaptured logs the captured std output from the shared libraries.
func LogCaptured(out string) {
	if out == "" {
		slog.Debug("No logs captured")
		return
	}
	entries := strings.Split(strings.ReplaceAll(out, "\r\n", "\n"), "\n")
	for _, entry := range entries {
		if groups := libLogRegex.FindStringSubmatch(entry); len(groups) == 5 {
			switch groups[2] {
			case "ERR":
				slog.Error(groups[4])
			case "INF":
				slog.Info(groups[4])
			default:
				slog.Debug(groups[4])
			}
		}
	}
}
