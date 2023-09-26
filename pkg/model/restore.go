package model

import (
	"encoding/json"
	"errors"
)

// RestoreRequest represents a restore operation request.
type RestoreRequest struct {
	Host               *string  `json:"host,omitempty"`
	Port               *int     `json:"port,omitempty"`
	User               *string  `json:"user,omitempty"`
	Password           *string  `json:"password,omitempty"`
	Directory          *string  `json:"directory,omitempty"`
	File               *string  `json:"file,omitempty"`
	S3Region           *string  `json:"s3_region,omitempty"`
	S3Profile          *string  `json:"s3_profile,omitempty"`
	S3EndpointOverride *string  `json:"s3_endpoint_override,omitempty"`
	S3LogLevel         *string  `json:"s3_log_level,omitempty"`
	NsList             []string `json:"ns_list,omitempty"`
	SetList            []string `json:"set_list,omitempty"`
	BinList            []string `json:"bin_list,omitempty"`
	Replace            *bool    `json:"replace,omitempty"`
	Unique             *bool    `json:"unique,omitempty"`
	NoGeneration       *bool    `json:"no_generation,omitempty"`
}

// Validate validates the restore operation request.
func (r *RestoreRequest) Validate() error {
	if r.Directory != nil && r.File != nil {
		return errors.New("both restore directory and file are specified")
	}
	if r.Directory == nil && r.File == nil {
		return errors.New("none of directory or file is specified")
	}
	if r.Host == nil {
		return errors.New("host is not specified")
	}
	if r.Port == nil {
		return errors.New("port is not specified")
	}
	return nil
}

// String satisfies the fmt.Stringer interface.
func (r RestoreRequest) String() string {
	request, err := json.Marshal(r)
	if err != nil {
		return err.Error()
	}
	return string(request)
}
