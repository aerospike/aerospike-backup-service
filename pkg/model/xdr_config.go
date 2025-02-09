package model

type XDRConfig struct {
	Enabled       bool
	DC            string
	LocalHost     string
	LocalPort     int
	Rewind        string
	MaxConns      int
	ReadTimeout   int64
	WriteTimeout  int64
	StartTimeout  int64
	PollingPeriod int64
}
