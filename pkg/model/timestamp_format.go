package model

// TimestampFormat represents the encoding for backup date timestamp in human-readable format.
type TimestampFormat string

const (
	TimestampFormatISO TimestampFormat = "ISO"
	TimestampFormatUS  TimestampFormat = "US"
	TimestampFormatEU  TimestampFormat = "EU"
)

// String returns the wire value of the timestamp format.
func (f TimestampFormat) String() string {
	return string(f)
}

var TimestampFormatPresets = map[TimestampFormat]string{
	TimestampFormatISO: "2006-01-02T15-04-05",
	TimestampFormatUS:  "Jan-02-2006-15-04-05",
	TimestampFormatEU:  "02-Jan-2006-15-04-05",
}
