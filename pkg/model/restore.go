package model

// RestoreRequest represents a restore operation request.
type RestoreRequest struct {
	Host         *string  `json:"host"`
	Port         *int     `json:"port"`
	User         *string  `json:"user"`
	Password     *string  `json:"password"`
	Directory    *string  `json:"directory"`
	NsList       []string `json:"ns_list"`
	SetList      []string `json:"set_list"`
	BinList      []string `json:"bin_list"`
	Replace      *bool    `json:"replace"`
	Unique       *bool    `json:"unique"`
	NoGeneration *bool    `json:"no_generation"`
}
