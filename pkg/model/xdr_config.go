package model

type XDRConfig struct {
	LocalHost     string
	Rewind        string
	MaxConns      *int
	ReadTimeout   *int64
	WriteTimeout  *int64
	StartTimeout  *int64
	PollingPeriod *int64
}
