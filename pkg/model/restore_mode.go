package model

// RestoreMode selects how the server restores a server-side backup.
type RestoreMode string

const (
	// RestoreModeStrict is the server's cold restore: the server takes the namespace from clients,
	// clears it and loads it from the backups, so the namespace holds exactly the backed-up data.
	RestoreModeStrict RestoreMode = "strict"
	// RestoreModeFuzzy writes the backed-up records into the live namespace as ordinary writes:
	// clients keep using it, and records the backups do not hold stay.
	RestoreModeFuzzy RestoreMode = "fuzzy"
)
