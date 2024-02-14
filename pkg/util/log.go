package util

import (
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"strings"
)

const (
	LevelTrace slog.Level = -8
)

var libLogRegex = regexp.MustCompile(`^(.+)\s\[(\D+)\]\s\[\s*(\d+)\]\s(.*)$`)

var ignoredLinesInDocker = []*regexp.Regexp{
	regexp.MustCompile("Failed to (open|read) /proc/sys/net/core/[rw]mem_max"),
}

var ignoredLines []*regexp.Regexp

var isDocker = isRunningInDockerContainer()

// LogCaptured logs the captured std output from the shared libraries.
func LogCaptured(out string) {
	if out == "" {
		slog.Debug("No logs captured")
		return
	}
	entries := strings.Split(strings.ReplaceAll(out, "\r\n", "\n"), "\n")
	for _, entry := range entries {
		if shouldSkip(entry) {
			continue
		}

		if groups := libLogRegex.FindStringSubmatch(entry); len(groups) == 5 {
			switch groups[2] {
			case "ERR":
				slog.Error(groups[4])
			case "INF":
				slog.Info(groups[4])
			default:
				slog.Debug(groups[4])
			}
		} else { // print to stderr
			fmt.Fprintln(os.Stderr, entry)
		}
	}
}

func shouldSkip(line string) bool {
	return strings.TrimSpace(line) == "" ||
		isDocker && matchesAnyPattern(line, ignoredLinesInDocker) ||
		matchesAnyPattern(line, ignoredLines)
}

func matchesAnyPattern(entry string, regexArray []*regexp.Regexp) bool {
	for _, regex := range regexArray {
		if regex.MatchString(entry) {
			return true
		}
	}
	return false
}
