package model

import "strings"

// TimestampFormat represents the encoding for backup date timestamp in human-readable format.
type TimestampFormat string

const (
	TimestampFormatISO TimestampFormat = "ISO"
	TimestampFormatUS  TimestampFormat = "US"
	TimestampFormatEU  TimestampFormat = "EU"
)

func TimestampFormatFromString(s string) TimestampFormat {
	return TimestampFormat(strings.ToUpper(s))
}

var TimestampFormatPresets = map[TimestampFormat]string{
	TimestampFormatISO: "2006-01-02T15-04-05",
	TimestampFormatUS:  "Jan-02-2006-15-04-05",
	TimestampFormatEU:  "02-Jan-2006-15-04-05",
}
