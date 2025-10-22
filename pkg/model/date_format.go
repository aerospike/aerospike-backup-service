package model

import "strings"

// DateFormat represents the encoding for backup date in human-readable format.
type DateFormat string

const (
	DateFormatISO DateFormat = "ISO"
	DateFormatUS  DateFormat = "US"
	DateFormatEU  DateFormat = "EU"
)

func DateFormatFromString(s string) DateFormat {
	return DateFormat(strings.ToUpper(s))
}

var DateFormatPresets = map[DateFormat]string{
	DateFormatISO: "2006-01-02T15-04-05",
	DateFormatUS:  "Jan-02-2006-15-04-05",
	DateFormatEU:  "02-Jan-2006-15-04-05",
}
